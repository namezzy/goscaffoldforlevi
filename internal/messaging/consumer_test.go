package messaging_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/example/go-enterprise/internal/messaging"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testEvent(t *testing.T) messaging.Event {
	t.Helper()
	event, err := messaging.NewEvent("user.created.v1", map[string]string{"user_id": "u1"})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	return event
}

// fakeDedupeStore models the property the consumer actually depends on: the
// dedupe claim and the side effect share a transaction, so a failed attempt
// leaves no claim behind.
//
// A fake that recorded the claim unconditionally would make every retry test
// pass while hiding the bug that matters — an event marked processed by an
// attempt that failed is an event whose work never happens and never will.
type fakeDedupeStore struct {
	mu       sync.Mutex
	claimed  map[string]bool
	dead     []deadLetter
	claimErr error
	deadErr  error
	// inTx guards against the consumer calling ClaimEvent outside WithTx,
	// which the real store rejects.
	inTx bool
}

type deadLetter struct {
	event    messaging.Event
	consumer string
	attempts int
	cause    string
}

func newFakeDedupeStore() *fakeDedupeStore {
	return &fakeDedupeStore{claimed: make(map[string]bool)}
}

func (f *fakeDedupeStore) WithTx(ctx context.Context, fn func(context.Context) error) error {
	f.mu.Lock()
	snapshot := make(map[string]bool, len(f.claimed))
	for k, v := range f.claimed {
		snapshot[k] = v
	}
	f.inTx = true
	f.mu.Unlock()

	err := fn(ctx)

	f.mu.Lock()
	f.inTx = false
	if err != nil {
		f.claimed = snapshot
	}
	f.mu.Unlock()
	return err
}

func (f *fakeDedupeStore) ClaimEvent(_ context.Context, id uuid.UUID, consumer string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.inTx {
		return false, errors.New("ClaimEvent called outside a transaction")
	}
	if f.claimErr != nil {
		return false, f.claimErr
	}
	key := id.String() + "/" + consumer
	if f.claimed[key] {
		return false, nil
	}
	f.claimed[key] = true
	return true, nil
}

func (f *fakeDedupeStore) RecordDeadLetter(
	_ context.Context, event messaging.Event, consumer string, attempts int, cause error,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deadErr != nil {
		return f.deadErr
	}
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	f.dead = append(f.dead, deadLetter{
		event: event, consumer: consumer, attempts: attempts, cause: message,
	})
	return nil
}

func (f *fakeDedupeStore) deadLetters() []deadLetter {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]deadLetter(nil), f.dead...)
}

// fastConsumer builds a consumer whose backoff is immediate, so the retry
// schedule can be asserted without spending real seconds on it. The schedule
// itself is tested separately in TestBackoffStaysWithinItsCeiling.
func fastConsumer(
	t *testing.T, handler messaging.Handler, store messaging.DedupeStore,
) *messaging.Consumer {
	t.Helper()
	cfg := messaging.DefaultConsumerConfig("test-consumer")
	cfg.BaseBackoff = time.Microsecond
	cfg.MaxBackoff = time.Microsecond
	return messaging.NewConsumer(handler, store, cfg, discardLogger())
}

// TestRedeliveryDoesNotRepeatTheSideEffect is the assertion that makes
// at-least-once delivery usable. The broker is allowed to deliver an event
// more than once — the relay itself can produce a duplicate by crashing
// between a successful publish and the row update — so a consumer that is not
// idempotent will double-charge, double-email or double-provision.
func TestRedeliveryDoesNotRepeatTheSideEffect(t *testing.T) {
	t.Parallel()

	store := newFakeDedupeStore()
	calls := 0
	consumer := fastConsumer(t, messaging.HandlerFunc(
		func(context.Context, messaging.Event) error {
			calls++
			return nil
		}), store)

	event := testEvent(t)
	for i := 0; i < 3; i++ {
		if err := consumer.Process(context.Background(), event); err != nil {
			t.Fatalf("delivery %d: %v", i+1, err)
		}
	}

	if calls != 1 {
		t.Errorf("the handler ran %d times for three deliveries of one event, want 1", calls)
	}
	if letters := store.deadLetters(); len(letters) != 0 {
		t.Errorf("a successfully handled event was dead-lettered: %+v", letters)
	}
}

// TestTwoConsumersEachProcessTheSameEvent checks the other half of dedupe: the
// log is keyed by consumer as well as event, so a second consumer of the same
// event is not silenced by the first. Keying on the event alone would make
// adding a consumer to an existing event type quietly do nothing.
func TestTwoConsumersEachProcessTheSameEvent(t *testing.T) {
	t.Parallel()

	store := newFakeDedupeStore()
	event := testEvent(t)

	ran := map[string]int{}
	for _, name := range []string{"mailer", "auditor"} {
		cfg := messaging.DefaultConsumerConfig(name)
		cfg.BaseBackoff = time.Microsecond
		consumer := messaging.NewConsumer(messaging.HandlerFunc(
			func(context.Context, messaging.Event) error {
				ran[name]++
				return nil
			}), store, cfg, discardLogger())

		if err := consumer.Process(context.Background(), event); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}

	if ran["mailer"] != 1 || ran["auditor"] != 1 {
		t.Errorf("each consumer should have run once, got %v", ran)
	}
}

// TestAFailedAttemptLeavesNoClaimBehind covers the reason the claim and the
// side effect share a transaction. If the claim survived a failed attempt, the
// retry would see an already-processed event, skip it, and the work would
// never happen — while every log line said the event was handled.
func TestAFailedAttemptLeavesNoClaimBehind(t *testing.T) {
	t.Parallel()

	store := newFakeDedupeStore()
	attempts := 0
	consumer := fastConsumer(t, messaging.HandlerFunc(
		func(context.Context, messaging.Event) error {
			attempts++
			if attempts < 3 {
				return errors.New("the mail provider is down")
			}
			return nil
		}), store)

	if err := consumer.Process(context.Background(), testEvent(t)); err != nil {
		t.Fatalf("Process: %v", err)
	}

	if attempts != 3 {
		t.Errorf("the handler ran %d times, want 3 (two failures then a success)", attempts)
	}
	if letters := store.deadLetters(); len(letters) != 0 {
		t.Errorf("an event that eventually succeeded was dead-lettered: %+v", letters)
	}
}

// TestExhaustedRetriesArePutInTheDeadLetterTable checks that a failing event
// leaves the retry loop instead of occupying it forever.
func TestExhaustedRetriesArePutInTheDeadLetterTable(t *testing.T) {
	t.Parallel()

	store := newFakeDedupeStore()
	attempts := 0
	consumer := fastConsumer(t, messaging.HandlerFunc(
		func(context.Context, messaging.Event) error {
			attempts++
			return errors.New("still broken")
		}), store)

	event := testEvent(t)
	// nil, not an error: the event has been dealt with. Returning an error
	// would tell the caller to redeliver a message that is already parked.
	if err := consumer.Process(context.Background(), event); err != nil {
		t.Fatalf("Process returned an error for a parked event: %v", err)
	}

	if attempts != messaging.DefaultConsumerConfig("x").MaxAttempts {
		t.Errorf("the handler ran %d times, want %d",
			attempts, messaging.DefaultConsumerConfig("x").MaxAttempts)
	}

	letters := store.deadLetters()
	if len(letters) != 1 {
		t.Fatalf("dead letters = %d, want 1", len(letters))
	}
	if letters[0].event.ID != event.ID {
		t.Errorf("the dead letter is for event %s, want %s", letters[0].event.ID, event.ID)
	}
	if letters[0].cause == "" {
		t.Error("the dead letter records no reason, which makes it untriageable")
	}
	if letters[0].attempts != attempts {
		t.Errorf("the dead letter says %d attempts, want %d", letters[0].attempts, attempts)
	}
}

// TestAPermanentFailureSkipsTheRetries checks that an error which cannot
// improve is parked at once. Retrying a malformed payload five times delays
// the alert without any chance of a different outcome.
func TestAPermanentFailureSkipsTheRetries(t *testing.T) {
	t.Parallel()

	store := newFakeDedupeStore()
	attempts := 0
	consumer := fastConsumer(t, messaging.HandlerFunc(
		func(context.Context, messaging.Event) error {
			attempts++
			return messaging.Permanent(errors.New("payload is not the shape this handler expects"))
		}), store)

	if err := consumer.Process(context.Background(), testEvent(t)); err != nil {
		t.Fatalf("Process: %v", err)
	}

	if attempts != 1 {
		t.Errorf("a permanent failure was retried: the handler ran %d times, want 1", attempts)
	}
	if len(store.deadLetters()) != 1 {
		t.Errorf("a permanently failed event was not parked")
	}
}

// TestAPanickingHandlerDoesNotKillTheConsumer covers the blast radius of a
// bug. One message with an unexpected shape must not stop every other message
// in the system, which is what an unrecovered panic in the consume loop does.
func TestAPanickingHandlerDoesNotKillTheConsumer(t *testing.T) {
	t.Parallel()

	store := newFakeDedupeStore()
	consumer := fastConsumer(t, messaging.HandlerFunc(
		func(_ context.Context, e messaging.Event) error {
			var payload struct {
				Nested *struct{ Value string }
			}
			_ = json.Unmarshal(e.Payload, &payload)
			return errors.New(payload.Nested.Value) // nil dereference, on purpose
		}), store)

	err := consumer.Process(context.Background(), testEvent(t))
	if err != nil {
		t.Fatalf("a panic escaped as an error instead of being parked: %v", err)
	}

	letters := store.deadLetters()
	if len(letters) != 1 {
		t.Fatalf("dead letters = %d, want 1", len(letters))
	}
	if letters[0].attempts != 1 {
		t.Errorf("a panic was retried %d times; it is a bug, and it will panic again",
			letters[0].attempts)
	}
}

// TestShutdownDoesNotParkAnUnfinishedEvent covers the deploy path. When the
// process is asked to stop mid-retry, the event has not failed — it was
// interrupted. Parking it would turn every rolling restart into a pile of dead
// letters that a human has to triage.
func TestShutdownDoesNotParkAnUnfinishedEvent(t *testing.T) {
	t.Parallel()

	store := newFakeDedupeStore()
	ctx, cancel := context.WithCancel(context.Background())

	consumer := fastConsumer(t, messaging.HandlerFunc(
		func(context.Context, messaging.Event) error {
			cancel() // the shutdown arrives while the handler is running
			return errors.New("interrupted")
		}), store)

	err := consumer.Process(ctx, testEvent(t))
	if err == nil {
		t.Fatal("Process reported success for an event it did not finish; " +
			"the message would be acknowledged and lost")
	}
	if letters := store.deadLetters(); len(letters) != 0 {
		t.Errorf("an interrupted event was parked as a dead letter: %+v", letters)
	}

	// Cancelling during the last attempt is the case that actually needs the
	// check. Earlier attempts are followed by a backoff, and the sleep notices
	// the cancellation on its own — so a version of the consumer with no
	// cancellation check at all still passes the first half of this test. On
	// the final attempt there is no sleep left to catch it, and the event goes
	// straight to the dead letter table because the process was stopping.
	t.Run("on the final attempt", func(t *testing.T) {
		t.Parallel()

		store := newFakeDedupeStore()
		ctx, cancel := context.WithCancel(context.Background())

		cfg := messaging.DefaultConsumerConfig("last-attempt")
		cfg.MaxAttempts = 1
		cfg.BaseBackoff = time.Microsecond
		consumer := messaging.NewConsumer(messaging.HandlerFunc(
			func(context.Context, messaging.Event) error {
				cancel()
				return errors.New("interrupted")
			}), store, cfg, discardLogger())

		if err := consumer.Process(ctx, testEvent(t)); err == nil {
			t.Fatal("Process reported success for an interrupted final attempt")
		}
		if letters := store.deadLetters(); len(letters) != 0 {
			t.Errorf("a shutdown parked an event as a dead letter: %+v", letters)
		}
	})
}

// TestBackoffStaysWithinItsCeiling checks the retry schedule directly, since
// the other tests deliberately run with the delays turned down.
//
// Full jitter means the delay is uniform over [0, ceiling), not the ceiling
// itself. The difference matters after a shared outage: without jitter every
// consumer that queued up retries in the same instant and knocks the recovered
// dependency straight back over.
func TestBackoffStaysWithinItsCeiling(t *testing.T) {
	t.Parallel()

	cfg := messaging.ConsumerConfig{
		Name:           "sched",
		HandlerTimeout: time.Second,
		MaxAttempts:    6,
		BaseBackoff:    100 * time.Millisecond,
		MaxBackoff:     time.Second,
	}

	delays := make([][]time.Duration, cfg.MaxAttempts)
	consumer := messaging.NewConsumer(
		messaging.HandlerFunc(func(context.Context, messaging.Event) error {
			return errors.New("nope")
		}),
		newFakeDedupeStore(), cfg, discardLogger())

	attempt := 0
	messaging.SetSleepForTest(consumer, func(_ context.Context, d time.Duration) error {
		delays[attempt] = append(delays[attempt], d)
		attempt++
		return nil
	})

	if err := consumer.Process(context.Background(), testEvent(t)); err != nil {
		t.Fatalf("Process: %v", err)
	}

	ceilings := []time.Duration{
		100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond,
		800 * time.Millisecond, time.Second, // capped by MaxBackoff
	}
	for i, ceiling := range ceilings[:cfg.MaxAttempts-1] {
		if len(delays[i]) != 1 {
			t.Fatalf("attempt %d slept %d times, want 1", i+1, len(delays[i]))
		}
		if got := delays[i][0]; got < 0 || got >= ceiling {
			t.Errorf("attempt %d slept %s, want a value in [0, %s)", i+1, got, ceiling)
		}
	}
	if len(delays[cfg.MaxAttempts-1]) != 0 {
		t.Error("the consumer slept after its final attempt instead of parking the event")
	}
}

// TestProcessReportsAnErrorWhenItCanNeitherHandleNorPark covers the one case
// where the caller must not acknowledge the message. A database that cannot
// even record a dead letter is not a message that has been dealt with, and
// acknowledging it would drop the event silently.
func TestProcessReportsAnErrorWhenItCanNeitherHandleNorPark(t *testing.T) {
	t.Parallel()

	store := newFakeDedupeStore()
	store.deadErr = errors.New("the database is unreachable")

	consumer := fastConsumer(t, messaging.HandlerFunc(
		func(context.Context, messaging.Event) error {
			return messaging.Permanent(errors.New("bad payload"))
		}), store)

	if err := consumer.Process(context.Background(), testEvent(t)); err == nil {
		t.Fatal("Process reported success although the event was neither handled nor parked")
	}
}
