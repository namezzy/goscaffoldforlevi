package messaging_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/example/go-enterprise/internal/messaging"
	"github.com/example/go-enterprise/internal/telemetry"
)

// recorder captures Observer calls in order.
type recorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *recorder) record(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, s)
}

func (r *recorder) MessagingPublished(eventType string) {
	r.record("published:" + eventType)
}

func (r *recorder) MessagingConsumed(eventType, consumer string, seconds float64, err error) {
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}
	r.record(fmt.Sprintf("consumed:%s:%s:%s:%t",
		eventType, consumer, outcome, seconds >= 0))
}

func (r *recorder) MessagingRetry(eventType, consumer string) {
	r.record("retry:" + eventType + ":" + consumer)
}

func (r *recorder) MessagingDeadLettered(eventType, consumer string) {
	r.record("dead:" + eventType + ":" + consumer)
}

func (r *recorder) MessagingDuplicate(eventType, consumer string) {
	r.record("duplicate:" + eventType + ":" + consumer)
}

func (r *recorder) MessagingOutboxPending(n int64) {
	r.record(fmt.Sprintf("pending:%d", n))
}

func (r *recorder) got() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

func (r *recorder) count(prefix string) int {
	n := 0
	for _, c := range r.got() {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func observedEvent() messaging.Event {
	return messaging.Event{
		ID:      uuid.New(),
		Type:    "user.created",
		Payload: []byte(`{}`),
	}
}

// The Observer interface exists so that internal/messaging never imports a
// metrics library: ADR-0001 puts instrumentation in technical packages, and
// Phase 8 audits for exactly this import. The compile-time assertion pins the
// other half — that *telemetry.Metrics satisfies it with no adapter in
// between, because an adapter is a place where a label can be dropped and
// nothing fails.
func TestTheMetricsRegistryIsAnObserver(t *testing.T) {
	var _ messaging.Observer = telemetry.NewMetrics()
}

// A consumer built without an observer must work. Instrumentation is optional
// by §5.1, and a nil interface field that is called is a panic in the message
// loop — the one place where a panic costs every message, not one request.
func TestAConsumerWithoutAnObserverStillProcesses(t *testing.T) {
	c := messaging.NewConsumer(
		messaging.HandlerFunc(func(context.Context, messaging.Event) error { return nil }),
		newFakeDedupeStore(), messaging.DefaultConsumerConfig("worker"), discardLogger())

	if err := c.Process(context.Background(), observedEvent()); err != nil {
		t.Fatalf("Process() = %v", err)
	}
}

func TestASuccessfulEventIsObservedOnce(t *testing.T) {
	rec := &recorder{}
	c := messaging.NewConsumer(
		messaging.HandlerFunc(func(context.Context, messaging.Event) error { return nil }),
		newFakeDedupeStore(), messaging.DefaultConsumerConfig("worker"), discardLogger())
	c.SetObserver(rec)

	if err := c.Process(context.Background(), observedEvent()); err != nil {
		t.Fatalf("Process() = %v", err)
	}

	want := []string{"consumed:user.created:worker:ok:true"}
	if got := rec.got(); !equalStrings(got, want) {
		t.Errorf("observed %v, want %v", got, want)
	}
}

// Retries have to be counted separately from failures. A handler that fails
// twice and succeeds on the third attempt is healthy; a handler that fails
// twice on every event is a system slowly falling behind. If both showed up
// only as a success at the end, the second would be invisible until the queue
// backed up.
func TestEachRetryIsObservedAndTheSuccessIsCountedOnce(t *testing.T) {
	rec := &recorder{}
	attempts := 0
	c := messaging.NewConsumer(
		messaging.HandlerFunc(func(context.Context, messaging.Event) error {
			attempts++
			if attempts < 3 {
				return errors.New("transient")
			}
			return nil
		}),
		newFakeDedupeStore(), messaging.DefaultConsumerConfig("worker"), discardLogger())
	c.SetObserver(rec)
	messaging.SetSleepForTest(c, func(context.Context, time.Duration) error { return nil })

	if err := c.Process(context.Background(), observedEvent()); err != nil {
		t.Fatalf("Process() = %v", err)
	}

	if got := rec.count("retry:"); got != 2 {
		t.Errorf("observed %d retries, want 2", got)
	}
	if got := rec.count("consumed:"); got != 1 {
		t.Errorf("observed %d terminal outcomes, want 1", got)
	}
	if got := rec.count("consumed:user.created:worker:ok:"); got != 1 {
		t.Errorf("the terminal outcome was not a success: %v", rec.got())
	}
	if got := rec.count("dead:"); got != 0 {
		t.Errorf("observed %d dead letters, want 0", got)
	}
}

// A dead letter must raise both counters: the DLQ gauge is what pages, and the
// failed outcome is what makes the consumer's error rate honest. Counting only
// the DLQ would leave the error rate at zero for a consumer failing everything.
func TestADeadLetterIsObservedAsBothADeadLetterAndAFailure(t *testing.T) {
	rec := &recorder{}
	c := messaging.NewConsumer(
		messaging.HandlerFunc(func(context.Context, messaging.Event) error {
			return fmt.Errorf("bad payload: %w", messaging.ErrPermanent)
		}),
		newFakeDedupeStore(), messaging.DefaultConsumerConfig("worker"), discardLogger())
	c.SetObserver(rec)

	if err := c.Process(context.Background(), observedEvent()); err != nil {
		t.Fatalf("Process() = %v", err)
	}

	if got := rec.count("dead:user.created:worker"); got != 1 {
		t.Errorf("observed %d dead letters, want 1: %v", got, rec.got())
	}
	if got := rec.count("consumed:user.created:worker:error:"); got != 1 {
		t.Errorf("the dead letter was not counted as a failure: %v", rec.got())
	}
}

// Found by a mutation that survived: reporting the exhausted-retries outcome
// as a success passed every test, because the only dead-letter test used
// ErrPermanent, which leaves Process through a different branch.
//
// Exhaustion is the commoner path in production. A permanent failure is a
// malformed payload, which is rare and usually one event; exhaustion is a
// dependency that stayed down for the whole retry schedule, which is an
// outage affecting every event at once. Having that report as success would
// leave the consumer's error rate at zero for the one incident it exists for.
func TestExhaustedRetriesAreObservedAsAFailure(t *testing.T) {
	rec := &recorder{}
	c := messaging.NewConsumer(
		messaging.HandlerFunc(func(context.Context, messaging.Event) error {
			return errors.New("dependency is down")
		}),
		newFakeDedupeStore(), messaging.DefaultConsumerConfig("worker"), discardLogger())
	c.SetObserver(rec)
	messaging.SetSleepForTest(c, func(context.Context, time.Duration) error { return nil })

	if err := c.Process(context.Background(), observedEvent()); err != nil {
		t.Fatalf("Process() = %v", err)
	}

	if got := rec.count("consumed:user.created:worker:error:"); got != 1 {
		t.Errorf("exhaustion was not counted as a failure: %v", rec.got())
	}
	if got := rec.count("consumed:user.created:worker:ok:"); got != 0 {
		t.Errorf("exhaustion was counted as a success: %v", rec.got())
	}
	if got := rec.count("dead:user.created:worker"); got != 1 {
		t.Errorf("observed %d dead letters, want 1: %v", got, rec.got())
	}
	if got, want := rec.count("retry:"), messaging.DefaultConsumerConfig("worker").MaxAttempts-1; got != want {
		t.Errorf("observed %d retries, want %d", got, want)
	}
}

// Shutdown is not a failure. Counting a cancelled attempt as an error means
// every rolling deploy shows up as a burst of consumer errors, and an alert
// that fires on every deploy is an alert nobody reads.
func TestAnEventInterruptedByShutdownIsNotObservedAsAFailure(t *testing.T) {
	rec := &recorder{}
	ctx, cancel := context.WithCancel(context.Background())
	c := messaging.NewConsumer(
		messaging.HandlerFunc(func(context.Context, messaging.Event) error {
			cancel()
			return errors.New("interrupted")
		}),
		newFakeDedupeStore(), messaging.DefaultConsumerConfig("worker"), discardLogger())
	c.SetObserver(rec)

	if err := c.Process(ctx, observedEvent()); err == nil {
		t.Fatal("Process() = nil, want the event left unacknowledged")
	}

	if got := rec.count("dead:"); got != 0 {
		t.Errorf("shutdown produced %d dead letters: %v", got, rec.got())
	}
	for _, call := range rec.got() {
		if strings.Contains(call, ":error:") {
			t.Errorf("shutdown was observed as a failure: %v", rec.got())
		}
	}
}

// A redelivery of an event this consumer already handled is the normal, safe
// outcome of at-least-once. It must be distinguishable from real work: if
// duplicates were counted as successes, a broker redelivering everything twice
// would look like twice the throughput rather than a problem.
func TestADuplicateIsObservedSeparatelyFromWork(t *testing.T) {
	rec := &recorder{}
	store := newFakeDedupeStore()
	event := observedEvent()
	cfg := messaging.DefaultConsumerConfig("worker")
	handler := messaging.HandlerFunc(func(context.Context, messaging.Event) error { return nil })

	first := messaging.NewConsumer(handler, store, cfg, discardLogger())
	first.SetObserver(rec)
	if err := first.Process(context.Background(), event); err != nil {
		t.Fatalf("first Process() = %v", err)
	}

	second := messaging.NewConsumer(handler, store, cfg, discardLogger())
	second.SetObserver(rec)
	if err := second.Process(context.Background(), event); err != nil {
		t.Fatalf("second Process() = %v", err)
	}

	if got := rec.count("consumed:user.created:worker:ok:"); got != 1 {
		t.Errorf("the duplicate was counted as work: %v", rec.got())
	}
	if got := rec.count("duplicate:"); got != 1 {
		t.Errorf("observed %d duplicates, want 1: %v", got, rec.got())
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The publish counter must be raised where the event actually reaches the
// broker, not where it is written to the outbox. Counting the write would make
// the metric agree with the database and disagree with reality, so a relay
// that had stopped delivering entirely would still show a healthy publish rate
// â which is precisely the outage the metric exists to reveal.
func TestOnlyEventsThatReachTheBrokerAreCountedAsPublished(t *testing.T) {
	delivered, rejected := testEvent(t), testEvent(t)
	outbox := newFakeOutbox(delivered, rejected)
	publisher := &selectivePublisher{
		reject: map[uuid.UUID]error{rejected.ID: errors.New("broker refused")},
	}

	rec := &recorder{}
	relay := messaging.NewRelay(outbox, publisher, fastRelayConfig(), discardLogger())
	relay.SetObserver(rec)

	runRelayUntil(t, relay, func() bool { return len(outbox.unpublished()) == 1 })

	if got := rec.count("published:"); got != 1 {
		t.Errorf("observed %d publishes for one delivered and one rejected event: %v",
			got, rec.got())
	}
}

// A relay without an observer must run. Instrumentation is optional, and a nil
// interface on the delivery path would stop every event in the system.
func TestARelayWithoutAnObserverStillDelivers(t *testing.T) {
	event := testEvent(t)
	outbox := newFakeOutbox(event)
	relay := messaging.NewRelay(outbox, &selectivePublisher{}, fastRelayConfig(), discardLogger())

	runRelayUntil(t, relay, func() bool { return len(outbox.unpublished()) == 0 })
}

// The outbox depth is the producer-side backlog, and it is the only signal
// that distinguishes "nothing is happening" from "nothing is getting out".
// A stalled relay publishes nothing, so every rate metric reads zero — exactly
// as it does on a quiet Sunday. The gauge is what tells the two apart.
//
// It is reported after every sweep rather than on a timer of its own, because
// a separate ticker is a second thing that can stop: if the relay is wedged,
// a gauge that keeps updating would keep reporting the last value it read.
func TestTheOutboxDepthIsReportedAfterEverySweep(t *testing.T) {
	rec := &recorder{}
	outbox := newFakeOutbox(testEvent(t), testEvent(t))
	relay := messaging.NewRelay(outbox, &selectivePublisher{}, fastRelayConfig(), discardLogger())
	relay.SetObserver(rec)

	runRelayUntil(t, relay, func() bool {
		return len(outbox.unpublished()) == 0 && rec.count("pending:") > 0
	})

	var last string
	for _, call := range rec.got() {
		if strings.HasPrefix(call, "pending:") {
			last = call
		}
	}
	if last != "pending:0" {
		t.Errorf("the last reported outbox depth was %q, want pending:0 once the table drained: %v",
			last, rec.got())
	}
}
