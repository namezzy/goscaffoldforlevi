//go:build integration

package messagingpg_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging/messagingpg"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/pgtest"
)

func newEvent(t *testing.T, payload string) messaging.Event {
	t.Helper()
	return messaging.Event{
		ID:         uuid.Must(uuid.NewV7()),
		Type:       "user.created.v1",
		OccurredAt: time.Now().UTC(),
		Payload:    json.RawMessage(payload),
	}
}

func countRows(t *testing.T, c *pgtest.Container, table string) int64 {
	t.Helper()
	var n int64
	err := c.Pool.QueryRow(context.Background(), `SELECT count(*) FROM `+table).Scan(&n)
	if err != nil {
		t.Fatalf("counting %s: %v", table, err)
	}
	return n
}

// TestTheOutboxWriteJoinsTheCallersTransaction is the entire reason the outbox
// exists. If the insert opened its own connection it would commit
// independently, and a business write that rolled back afterwards would leave
// an event announcing something that never happened.
func TestTheOutboxWriteJoinsTheCallersTransaction(t *testing.T) {
	c := pgtest.Start(t)
	ctx := context.Background()

	publisher := messagingpg.NewOutboxPublisher(c.Pool)
	txm := postgres.NewTxManager(c.Pool)

	sentinel := errors.New("the business rule rejected this")
	err := txm.WithTx(ctx, func(ctx context.Context) error {
		if err := publisher.Publish(ctx, newEvent(t, `{"email":"ada@example.com"}`)); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("WithTx = %v, want the caller's error", err)
	}

	if n := countRows(t, c, "outbox_events"); n != 0 {
		t.Fatalf("%d events survived a rolled-back transaction", n)
	}
}

// TestACommittedEventIsVisibleToTheRelay is the other half: what commits is
// there to be delivered.
func TestACommittedEventIsVisibleToTheRelay(t *testing.T) {
	c := pgtest.Start(t)
	ctx := context.Background()

	publisher := messagingpg.NewOutboxPublisher(c.Pool)
	txm := postgres.NewTxManager(c.Pool)
	store := messagingpg.NewStore(c.Pool, txm)

	event := newEvent(t, `{"email":"ada@example.com"}`)
	err := txm.WithTx(ctx, func(ctx context.Context) error {
		return publisher.Publish(ctx, event)
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}

	pending, err := store.PendingCount(ctx)
	if err != nil {
		t.Fatalf("PendingCount: %v", err)
	}
	if pending != 1 {
		t.Fatalf("PendingCount = %d, want 1", pending)
	}

	var claimed []messaging.PendingOutboxEvent
	err = store.ClaimBatch(ctx, 10, func(_ context.Context, batch []messaging.PendingOutboxEvent) error {
		claimed = batch
		return nil
	})
	if err != nil {
		t.Fatalf("ClaimBatch: %v", err)
	}
	if len(claimed) != 1 || claimed[0].Event.ID != event.ID {
		t.Fatalf("claimed %v, want the committed event %s", claimed, event.ID)
	}
	// The envelope must survive the round trip intact, or a consumer receives
	// an event it cannot interpret.
	if claimed[0].Event.Type != event.Type {
		t.Errorf("type = %q, want %q", claimed[0].Event.Type, event.Type)
	}
	// The payload is compared semantically rather than byte for byte: the
	// column is jsonb, which normalises whitespace and key order. That is
	// fine for consumers, which parse the JSON, but it does mean the exact
	// bytes are not preserved — anything that signed or hashed the raw
	// payload would break. STATE.md records this.
	var gotPayload, wantPayload map[string]any
	if err := json.Unmarshal(claimed[0].Event.Payload, &gotPayload); err != nil {
		t.Fatalf("the stored payload is not valid JSON: %v", err)
	}
	if err := json.Unmarshal(event.Payload, &wantPayload); err != nil {
		t.Fatalf("the source payload is not valid JSON: %v", err)
	}
	if !maps.Equal(toStringMap(t, gotPayload), toStringMap(t, wantPayload)) {
		t.Errorf("payload = %s, want the same content as %s",
			claimed[0].Event.Payload, event.Payload)
	}
	if claimed[0].Event.OccurredAt.Unix() != event.OccurredAt.Unix() {
		t.Errorf("occurred_at = %s, want %s", claimed[0].Event.OccurredAt, event.OccurredAt)
	}
}

// TestTwoRelaysNeverClaimTheSameEvent is the concurrency guarantee that lets
// the worker be scaled past one replica. Without FOR UPDATE SKIP LOCKED every
// replica publishes every event, turning a horizontal scale-up into duplicate
// mail for every user.
func TestTwoRelaysNeverClaimTheSameEvent(t *testing.T) {
	c := pgtest.Start(t)
	ctx := context.Background()

	publisher := messagingpg.NewOutboxPublisher(c.Pool)
	txm := postgres.NewTxManager(c.Pool)

	const total = 20
	for i := 0; i < total; i++ {
		err := txm.WithTx(ctx, func(ctx context.Context) error {
			return publisher.Publish(ctx, newEvent(t, `{"n":1}`))
		})
		if err != nil {
			t.Fatalf("seeding event %d: %v", i, err)
		}
	}

	// Two stores on the same pool stand in for two worker replicas.
	storeA := messagingpg.NewStore(c.Pool, txm)
	storeB := messagingpg.NewStore(c.Pool, txm)

	var (
		mu      sync.Mutex
		claimed = map[uuid.UUID]int{}
		wg      sync.WaitGroup
	)

	claim := func(s *messagingpg.Store) {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			_ = s.ClaimBatch(ctx, 5, func(ctx context.Context, batch []messaging.PendingOutboxEvent) error {
				for _, pending := range batch {
					mu.Lock()
					claimed[pending.Event.ID]++
					mu.Unlock()
					// Marking inside the claim transaction is what the relay
					// does; it is what makes the row stop being claimable.
					if err := s.MarkPublished(ctx, pending.Event.ID); err != nil {
						return err
					}
				}
				return nil
			})
		}
	}

	wg.Add(2)
	go claim(storeA)
	go claim(storeB)
	wg.Wait()

	if len(claimed) != total {
		t.Errorf("claimed %d distinct events, want all %d", len(claimed), total)
	}
	for id, times := range claimed {
		if times != 1 {
			t.Errorf("event %s was claimed %d times; two relays delivered it", id, times)
		}
	}
}

// TestAClaimedEventIsInvisibleUntilItsTransactionEnds is the locking property
// stated directly. A second relay must see nothing while the first holds the
// row, and must see it again if the first rolls back.
func TestAClaimedEventIsInvisibleUntilItsTransactionEnds(t *testing.T) {
	c := pgtest.Start(t)
	ctx := context.Background()

	publisher := messagingpg.NewOutboxPublisher(c.Pool)
	txm := postgres.NewTxManager(c.Pool)
	store := messagingpg.NewStore(c.Pool, txm)

	event := newEvent(t, `{"n":1}`)
	if err := txm.WithTx(ctx, func(ctx context.Context) error {
		return publisher.Publish(ctx, event)
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	inside := make(chan struct{})
	release := make(chan struct{})
	secondSaw := make(chan int, 1)

	go func() {
		_ = store.ClaimBatch(ctx, 10, func(_ context.Context, batch []messaging.PendingOutboxEvent) error {
			if len(batch) != 1 {
				t.Errorf("the first claim saw %d events, want 1", len(batch))
			}
			close(inside)
			<-release
			// Rolling back must return the event to the queue: an event lost
			// because a relay crashed mid-publish is exactly what at-least-once
			// forbids.
			return errors.New("this relay crashed")
		})
	}()

	<-inside
	secondErr := make(chan error, 1)
	go func() {
		other := messagingpg.NewStore(c.Pool, txm)
		// ClaimBatch does not invoke the callback when nothing is claimable,
		// so "never called" and "called with an empty batch" both mean the
		// second relay saw nothing.
		seen := 0
		err := other.ClaimBatch(ctx, 10, func(_ context.Context, batch []messaging.PendingOutboxEvent) error {
			seen = len(batch)
			return nil
		})
		secondErr <- err
		secondSaw <- seen
	}()

	select {
	case seen := <-secondSaw:
		if err := <-secondErr; err != nil {
			t.Fatalf("the second claim failed: %v", err)
		}
		if seen != 0 {
			t.Errorf("a second relay claimed %d events while the first held the lock", seen)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the second claim blocked instead of skipping the locked row")
	}

	close(release)

	// After the rollback the event must be claimable again.
	deadline := time.Now().Add(10 * time.Second)
	for {
		pending, err := store.PendingCount(ctx)
		if err != nil {
			t.Fatalf("PendingCount: %v", err)
		}
		if pending == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("PendingCount = %d after a rolled-back claim, want the event back in the queue", pending)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestAPublishedEventIsNotClaimedAgain covers the mark. Without it the relay
// would republish the same event on every sweep for ever.
func TestAPublishedEventIsNotClaimedAgain(t *testing.T) {
	c := pgtest.Start(t)
	ctx := context.Background()

	publisher := messagingpg.NewOutboxPublisher(c.Pool)
	txm := postgres.NewTxManager(c.Pool)
	store := messagingpg.NewStore(c.Pool, txm)

	event := newEvent(t, `{"n":1}`)
	if err := txm.WithTx(ctx, func(ctx context.Context) error {
		return publisher.Publish(ctx, event)
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	err := store.ClaimBatch(ctx, 10, func(ctx context.Context, batch []messaging.PendingOutboxEvent) error {
		for _, p := range batch {
			if err := store.MarkPublished(ctx, p.Event.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}

	var second int
	err = store.ClaimBatch(ctx, 10, func(_ context.Context, batch []messaging.PendingOutboxEvent) error {
		second = len(batch)
		return nil
	})
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if second != 0 {
		t.Errorf("a published event was claimed again %d times", second)
	}
}

// TestTheSameEventIsProcessedOnlyOnce is the consumer-side guarantee. The
// broker delivers at least once, so the second delivery of an event must not
// run the handler's side effects again.
func TestTheSameEventIsProcessedOnlyOnce(t *testing.T) {
	c := pgtest.Start(t)
	ctx := context.Background()

	txm := postgres.NewTxManager(c.Pool)
	store := messagingpg.NewStore(c.Pool, txm)
	event := newEvent(t, `{"n":1}`)

	claim := func() (claimed bool) {
		err := store.WithTx(ctx, func(ctx context.Context) error {
			var err error
			claimed, err = store.ClaimEvent(ctx, event.ID, "welcome-email")
			return err
		})
		if err != nil {
			t.Fatalf("ClaimEvent: %v", err)
		}
		return claimed
	}

	if !claim() {
		t.Fatal("the first delivery was treated as a duplicate")
	}
	if claim() {
		t.Error("the second delivery of the same event was processed again")
	}
}

// TestADifferentConsumerStillProcessesTheEvent is the case a single-column
// primary key on event_id gets wrong. Keying deduplication on the event alone
// means that adding a second consumer to an existing event type silently does
// nothing at all — no error, no log line, just a feature that never runs.
func TestADifferentConsumerStillProcessesTheEvent(t *testing.T) {
	c := pgtest.Start(t)
	ctx := context.Background()

	txm := postgres.NewTxManager(c.Pool)
	store := messagingpg.NewStore(c.Pool, txm)
	event := newEvent(t, `{"n":1}`)

	claimFor := func(consumer string) (claimed bool) {
		err := store.WithTx(ctx, func(ctx context.Context) error {
			var err error
			claimed, err = store.ClaimEvent(ctx, event.ID, consumer)
			return err
		})
		if err != nil {
			t.Fatalf("ClaimEvent(%s): %v", consumer, err)
		}
		return claimed
	}

	if !claimFor("welcome-email") {
		t.Fatal("the first consumer did not claim the event")
	}
	if !claimFor("analytics") {
		t.Error("a second consumer was refused an event it had never seen")
	}
}

// TestTheDedupeRecordRollsBackWithTheHandler is what makes idempotency honest.
// If the record committed independently, a handler that failed after it was
// written would never run again: the event would be marked processed and its
// work would never have happened.
func TestTheDedupeRecordRollsBackWithTheHandler(t *testing.T) {
	c := pgtest.Start(t)
	ctx := context.Background()

	txm := postgres.NewTxManager(c.Pool)
	store := messagingpg.NewStore(c.Pool, txm)
	event := newEvent(t, `{"n":1}`)

	sentinel := errors.New("the handler failed after claiming")
	err := store.WithTx(ctx, func(ctx context.Context) error {
		claimed, err := store.ClaimEvent(ctx, event.ID, "welcome-email")
		if err != nil {
			return err
		}
		if !claimed {
			t.Fatal("the first claim was refused")
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("WithTx = %v, want the handler's error", err)
	}

	var retried bool
	err = store.WithTx(ctx, func(ctx context.Context) error {
		var err error
		retried, err = store.ClaimEvent(ctx, event.ID, "welcome-email")
		return err
	})
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if !retried {
		t.Error("the event was marked processed even though its handler failed and rolled back")
	}
}

// TestClaimingOutsideATransactionIsRefused guards against a caller that would
// otherwise get silent data loss. A dedupe record written outside the handler's
// transaction is not a weaker guarantee, it is no guarantee, so the mistake
// must be loud.
func TestClaimingOutsideATransactionIsRefused(t *testing.T) {
	c := pgtest.Start(t)
	ctx := context.Background()

	store := messagingpg.NewStore(c.Pool, postgres.NewTxManager(c.Pool))

	if _, err := store.ClaimEvent(ctx, uuid.Must(uuid.NewV7()), "welcome-email"); err == nil {
		t.Error("ClaimEvent succeeded outside a transaction; the dedupe record would not be atomic")
	}
}

// TestADeadLetteredEventCanBeReplayed closes the operational loop. An event
// that exhausted its retries must be recoverable once the bug is fixed, or the
// dead letter table is a graveyard rather than a queue.
func TestADeadLetteredEventCanBeReplayed(t *testing.T) {
	c := pgtest.Start(t)
	ctx := context.Background()

	txm := postgres.NewTxManager(c.Pool)
	store := messagingpg.NewStore(c.Pool, txm)
	event := newEvent(t, `{"email":"ada@example.com"}`)

	err := store.RecordDeadLetter(ctx, event, "welcome-email", 5,
		errors.New("the mail provider rejected the address"))
	if err != nil {
		t.Fatalf("RecordDeadLetter: %v", err)
	}

	page, err := store.ListDeadLetters(ctx, "", 10)
	if err != nil {
		t.Fatalf("ListDeadLetters: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("ListDeadLetters returned %d rows, want 1", len(page.Items))
	}
	if page.Total != 1 {
		t.Errorf("ListDeadLetters Total = %d, want 1", page.Total)
	}
	letters := page.Items
	// The reason must be stored, or an operator sees a failed event with no
	// way to tell why it failed.
	if letters[0].LastError == "" {
		t.Error("the dead letter records no reason for the failure")
	}

	// Replay is keyed on the dead-letter row rather than the event, because one
	// event can be dead-lettered by two different consumers and "replay the
	// event" would not say which of them should run again.
	if err := store.Replay(ctx, letters[0].ID); err != nil {
		t.Fatalf("Replay: %v", err)
	}

	pending, err := store.PendingCount(ctx)
	if err != nil {
		t.Fatalf("PendingCount: %v", err)
	}
	if pending != 1 {
		t.Errorf("PendingCount = %d after a replay, want the event back in the outbox", pending)
	}
}

// TestReplayWhileTheOriginalOutboxRowIsRetained is the regression test for
// replay failing with a duplicate key on outbox_events_pkey. A dead letter
// comes from an event that went through the outbox, and the delivered row is
// kept for PublishedRetention, so in practice the original row is still there
// when an operator replays. Replay inserted a second row under the same ID
// (the ID is kept on purpose, for consumer dedupe) and always failed inside
// the retention window. The test above missed it by never writing that row.
func TestReplayWhileTheOriginalOutboxRowIsRetained(t *testing.T) {
	c := pgtest.Start(t)
	ctx := context.Background()

	txm := postgres.NewTxManager(c.Pool)
	store := messagingpg.NewStore(c.Pool, txm)
	publisher := messagingpg.NewOutboxPublisher(c.Pool)
	event := newEvent(t, `{"email":"ada@example.com"}`)

	if err := txm.WithTx(ctx, func(ctx context.Context) error {
		return publisher.Publish(ctx, event)
	}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := store.MarkPublished(ctx, event.ID); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}
	if err := store.RecordDeadLetter(ctx, event, "welcome-email", 5,
		errors.New("the mail provider rejected the address")); err != nil {
		t.Fatalf("RecordDeadLetter: %v", err)
	}
	page, err := store.ListDeadLetters(ctx, "", 10)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("ListDeadLetters = %d rows, %v; want 1 row", len(page.Items), err)
	}

	if err := store.Replay(ctx, page.Items[0].ID); err != nil {
		t.Fatalf("Replay() with the original outbox row retained = %v, want nil", err)
	}

	pending, err := store.PendingCount(ctx)
	if err != nil {
		t.Fatalf("PendingCount: %v", err)
	}
	if pending != 1 {
		t.Errorf("PendingCount = %d after a replay, want the event back in the outbox", pending)
	}

	// The relay must see the replay as a new delivery, or the broker drops it
	// as a duplicate of the original publish (see natsmq's msgIDFor).
	var claimed []messaging.PendingOutboxEvent
	if err := store.ClaimBatch(ctx, 10, func(_ context.Context, events []messaging.PendingOutboxEvent) error {
		claimed = events
		return nil
	}); err != nil {
		t.Fatalf("ClaimBatch: %v", err)
	}
	if len(claimed) != 1 || claimed[0].Event.ID != event.ID || claimed[0].Event.ReplayGeneration != 1 {
		t.Fatalf("ClaimBatch after a replay = %+v, want the same event ID with ReplayGeneration 1", claimed)
	}
}

// TestRetentionDeletesOnlyWhatItShould checks the sweep does not take
// undelivered events with it. Pruning by age alone would silently discard a
// backlog that built up during a broker outage.
func TestRetentionDeletesOnlyWhatItShould(t *testing.T) {
	c := pgtest.Start(t)
	ctx := context.Background()

	publisher := messagingpg.NewOutboxPublisher(c.Pool)
	txm := postgres.NewTxManager(c.Pool)
	store := messagingpg.NewStore(c.Pool, txm)

	delivered := newEvent(t, `{"n":1}`)
	stuck := newEvent(t, `{"n":2}`)
	for _, e := range []messaging.Event{delivered, stuck} {
		event := e
		if err := txm.WithTx(ctx, func(ctx context.Context) error {
			return publisher.Publish(ctx, event)
		}); err != nil {
			t.Fatalf("seeding: %v", err)
		}
	}

	if err := store.MarkPublished(ctx, delivered.ID); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}
	// Age the published row past the retention window.
	_, err := c.Pool.Exec(ctx,
		`UPDATE outbox_events SET published_at = now() - interval '30 days' WHERE id = $1`,
		delivered.ID)
	if err != nil {
		t.Fatalf("ageing the row: %v", err)
	}

	deleted, err := store.PrunePublished(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("PrunePublished: %v", err)
	}
	if deleted != 1 {
		t.Errorf("pruned %d rows, want 1", deleted)
	}

	if pending, _ := store.PendingCount(ctx); pending != 1 {
		t.Errorf("PendingCount = %d, want the undelivered event untouched", pending)
	}
}

// toStringMap flattens a decoded payload for comparison.
func toStringMap(t *testing.T, m map[string]any) map[string]string {
	t.Helper()
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = fmt.Sprint(v)
	}
	return out
}
