package messaging_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
)

// fakeOutbox is an in-memory outbox that keeps the property the relay relies
// on: a claimed row stays claimable until it is explicitly marked published.
// Anything weaker would let a relay lose events without any test noticing.
type fakeOutbox struct {
	mu        sync.Mutex
	rows      []*outboxRow
	claimErr  error
	markErr   error
	pruned    []time.Time
	prunedDed []time.Time
}

type outboxRow struct {
	event     messaging.Event
	published bool
	attempts  int32
	lastError string
}

func newFakeOutbox(events ...messaging.Event) *fakeOutbox {
	o := &fakeOutbox{}
	for _, e := range events {
		o.rows = append(o.rows, &outboxRow{event: e})
	}
	return o
}

func (o *fakeOutbox) ClaimBatch(
	ctx context.Context,
	limit int32,
	fn func(ctx context.Context, events []messaging.PendingOutboxEvent) error,
) error {
	o.mu.Lock()
	if o.claimErr != nil {
		err := o.claimErr
		o.mu.Unlock()
		return err
	}

	var batch []messaging.PendingOutboxEvent
	for _, row := range o.rows {
		if row.published {
			continue
		}
		batch = append(batch, messaging.PendingOutboxEvent{
			Event: row.event, Attempts: row.attempts,
		})
		if int32(len(batch)) == limit {
			break
		}
	}
	o.mu.Unlock()

	// The real store skips the callback entirely when nothing is claimable.
	// A fake that called it with an empty batch would be a weaker contract
	// than production, and the relay would be tested against behaviour it
	// never actually meets.
	if len(batch) == 0 {
		return nil
	}
	return fn(ctx, batch)
}

func (o *fakeOutbox) MarkPublished(_ context.Context, id uuid.UUID) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.markErr != nil {
		return o.markErr
	}
	for _, row := range o.rows {
		if row.event.ID == id {
			row.published = true
			return nil
		}
	}
	return errors.New("no such outbox row")
}

func (o *fakeOutbox) RecordFailure(_ context.Context, id uuid.UUID, cause error) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, row := range o.rows {
		if row.event.ID == id {
			row.attempts++
			row.lastError = cause.Error()
			return nil
		}
	}
	return nil
}

func (o *fakeOutbox) PendingCount(context.Context) (int64, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	var pending int64
	for _, row := range o.rows {
		if !row.published {
			pending++
		}
	}
	return pending, nil
}

func (o *fakeOutbox) PrunePublished(_ context.Context, cutoff time.Time) (int64, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.pruned = append(o.pruned, cutoff)
	return 0, nil
}

func (o *fakeOutbox) PruneProcessed(_ context.Context, cutoff time.Time) (int64, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.prunedDed = append(o.prunedDed, cutoff)
	return 0, nil
}

func (o *fakeOutbox) unpublished() []messaging.Event {
	o.mu.Lock()
	defer o.mu.Unlock()
	var left []messaging.Event
	for _, row := range o.rows {
		if !row.published {
			left = append(left, row.event)
		}
	}
	return left
}

func (o *fakeOutbox) row(id uuid.UUID) outboxRow {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, r := range o.rows {
		if r.event.ID == id {
			return *r
		}
	}
	return outboxRow{}
}

func fastRelayConfig() messaging.RelayConfig {
	cfg := messaging.DefaultRelayConfig()
	cfg.PollInterval = time.Millisecond
	cfg.PruneInterval = time.Hour
	return cfg
}

func makeEvents(t *testing.T, n int) []messaging.Event {
	t.Helper()
	events := make([]messaging.Event, 0, n)
	for i := 0; i < n; i++ {
		events = append(events, testEvent(t))
	}
	return events
}

// TestTheRelayDeliversAndMarksEveryEvent is the base case: what is committed
// gets published exactly once and is not published again.
func TestTheRelayDeliversAndMarksEveryEvent(t *testing.T) {
	t.Parallel()

	events := makeEvents(t, 3)
	outbox := newFakeOutbox(events...)
	publisher := messaging.NewMemoryPublisher()

	relay := messaging.NewRelay(outbox, publisher, fastRelayConfig(), discardLogger())
	runRelayUntil(t, relay, func() bool { return len(outbox.unpublished()) == 0 })

	if got := len(publisher.Events()); got != 3 {
		t.Errorf("published %d events, want 3", got)
	}
	if left := outbox.unpublished(); len(left) != 0 {
		t.Errorf("%d events were left unpublished", len(left))
	}
}

// TestABurstLargerThanOneBatchIsDrainedImmediately covers a defect that is
// invisible in a small test and painful in production: without a drain loop
// the relay publishes one batch per poll interval, so a spike of ten thousand
// events at a batch size of a hundred takes a hundred seconds to clear while
// the relay sits idle between batches.
func TestABurstLargerThanOneBatchIsDrainedImmediately(t *testing.T) {
	t.Parallel()

	cfg := fastRelayConfig()
	cfg.BatchSize = 2
	// A poll interval long enough that a second tick cannot be what drains the
	// backlog. If the events all get out, it is because one wake-up drained
	// them.
	cfg.PollInterval = time.Hour

	events := makeEvents(t, 7)
	outbox := newFakeOutbox(events...)
	publisher := messaging.NewMemoryPublisher()

	relay := messaging.NewRelay(outbox, publisher, cfg, discardLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() { defer close(done); _ = relay.Run(ctx) }()

	relay.Notify()
	waitFor(t, func() bool { return len(outbox.unpublished()) == 0 })

	cancel()
	<-done

	if got := len(publisher.Events()); got != 7 {
		t.Errorf("published %d of 7 events in one wake-up", got)
	}
}

// TestOneUndeliverableEventDoesNotBlockTheBatch covers head-of-line blocking.
// A single message the broker rejects — too large, malformed subject — must
// not hold up every event queued behind it, because the queue only grows while
// nothing moves.
func TestOneUndeliverableEventDoesNotBlockTheBatch(t *testing.T) {
	t.Parallel()

	events := makeEvents(t, 3)
	poison := events[1]
	outbox := newFakeOutbox(events...)

	publisher := &selectivePublisher{
		reject: map[uuid.UUID]error{poison.ID: errors.New("message rejected by the broker")},
	}

	relay := messaging.NewRelay(outbox, publisher, fastRelayConfig(), discardLogger())
	runRelayUntil(t, relay, func() bool { return len(outbox.unpublished()) == 1 })

	left := outbox.unpublished()
	if len(left) != 1 || left[0].ID != poison.ID {
		t.Fatalf("unpublished = %v, want only the rejected event %s", left, poison.ID)
	}

	// The failure is counted and its reason recorded, or an operator has no
	// way to tell a stuck event from a slow one.
	row := outbox.row(poison.ID)
	if row.attempts == 0 {
		t.Error("the failed delivery was not counted")
	}
	if row.lastError == "" {
		t.Error("the failure reason was not recorded")
	}
}

// TestAnUndeliverableEventIsNeverDroppedFromTheOutbox states the rule that
// makes the outbox worth having. A broker outage must delay events, not
// discard them: an event deleted because it could not be delivered is an
// inconsistency nobody will ever detect.
func TestAnUndeliverableEventIsNeverDroppedFromTheOutbox(t *testing.T) {
	t.Parallel()

	event := testEvent(t)
	outbox := newFakeOutbox(event)
	publisher := &selectivePublisher{
		reject: map[uuid.UUID]error{event.ID: errors.New("broker is down")},
	}

	relay := messaging.NewRelay(outbox, publisher, fastRelayConfig(), discardLogger())
	runRelayUntil(t, relay, func() bool { return outbox.row(event.ID).attempts >= 3 })

	if left := outbox.unpublished(); len(left) != 1 {
		t.Fatalf("the undeliverable event was removed from the outbox; unpublished = %d", len(left))
	}
}

// TestStoppingTheRelayLeavesUndeliveredEventsInTheTable is the shutdown
// guarantee. The relay holds no state a restart would not rediscover, so
// stopping it mid-backlog costs latency and nothing else.
func TestStoppingTheRelayLeavesUndeliveredEventsInTheTable(t *testing.T) {
	t.Parallel()

	cfg := fastRelayConfig()
	cfg.BatchSize = 1
	cfg.PollInterval = time.Millisecond

	events := makeEvents(t, 5)
	outbox := newFakeOutbox(events...)

	// The publisher blocks after the first event, so the relay is guaranteed
	// to still have work when it is stopped.
	release := make(chan struct{})
	publisher := &blockingPublisher{after: 1, block: release}

	relay := messaging.NewRelay(outbox, publisher, cfg, discardLogger())
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() { defer close(done); _ = relay.Run(ctx) }()

	waitFor(t, func() bool { return publisher.count() >= 1 })
	cancel()
	close(release)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the relay did not stop within five seconds of being cancelled")
	}

	delivered := publisher.count()
	remaining := len(outbox.unpublished())
	if delivered+remaining != 5 {
		t.Errorf("%d events delivered and %d still in the table, want the two to sum to 5",
			delivered, remaining)
	}
}

// TestTheRetentionSweepRunsOnItsOwnSchedule checks that pruning is driven by
// its own ticker rather than by delivery, so a quiet service still cleans up.
func TestTheRetentionSweepRunsOnItsOwnSchedule(t *testing.T) {
	t.Parallel()

	cfg := fastRelayConfig()
	cfg.PollInterval = time.Hour // no delivery work at all
	cfg.PruneInterval = time.Millisecond
	cfg.PublishedRetention = 48 * time.Hour
	cfg.ProcessedRetention = 96 * time.Hour

	outbox := newFakeOutbox()
	relay := messaging.NewRelay(outbox, messaging.NewMemoryPublisher(), cfg, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); _ = relay.Run(ctx) }()

	waitFor(t, func() bool {
		outbox.mu.Lock()
		defer outbox.mu.Unlock()
		return len(outbox.pruned) > 0 && len(outbox.prunedDed) > 0
	})
	cancel()
	<-done

	outbox.mu.Lock()
	defer outbox.mu.Unlock()
	// The dedupe log must be kept longer than delivered events, or a
	// redelivery after the shorter window would be processed twice.
	if !outbox.prunedDed[0].Before(outbox.pruned[0]) {
		t.Errorf("processed events are pruned back to %s and published events to %s; "+
			"the dedupe log must be kept longer", outbox.prunedDed[0], outbox.pruned[0])
	}
}

// selectivePublisher fails for specific events and succeeds for the rest.
type selectivePublisher struct {
	mu       sync.Mutex
	reject   map[uuid.UUID]error
	accepted []messaging.Event
}

func (p *selectivePublisher) Publish(_ context.Context, e messaging.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err, rejected := p.reject[e.ID]; rejected {
		return err
	}
	p.accepted = append(p.accepted, e)
	return nil
}

// blockingPublisher accepts a number of events and then blocks, which is how
// the shutdown test guarantees the relay still has work in hand.
type blockingPublisher struct {
	mu        sync.Mutex
	published int
	after     int
	block     chan struct{}
}

func (p *blockingPublisher) Publish(ctx context.Context, _ messaging.Event) error {
	p.mu.Lock()
	p.published++
	shouldBlock := p.published > p.after
	p.mu.Unlock()

	if shouldBlock {
		select {
		case <-p.block:
			return errors.New("publisher stopped")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (p *blockingPublisher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	published := p.published
	if published > p.after {
		published = p.after
	}
	return published
}

// runRelayUntil runs a relay until done reports true, then stops it.
func runRelayUntil(t *testing.T, relay *messaging.Relay, done func() bool) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopped := make(chan struct{})
	go func() { defer close(stopped); _ = relay.Run(ctx) }()

	waitFor(t, done)
	cancel()

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("the relay did not stop within five seconds")
	}
}

// waitFor polls a condition instead of sleeping for a guessed duration. A
// fixed sleep is either slower than it needs to be or flaky on a loaded
// machine, and usually both.
func waitFor(t *testing.T, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition was not met within five seconds")
}
