//go:build integration

package natsmq_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging/natsmq"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// connect dials the broker started by `make broker-up`.
//
// The test skips rather than fails when no broker is reachable: a developer
// working on the HTTP API runs the default compose profile, and a suite that
// failed for them would train everyone to ignore it.
func connect(t *testing.T, stream, consumer string) *natsmq.Conn {
	t.Helper()

	url := os.Getenv("MESSAGING_NATS_URLS")
	if url == "" {
		url = "nats://localhost:4222"
	}

	cfg := natsmq.DefaultConfig()
	cfg.URLs = []string{url}
	cfg.StreamName = stream
	cfg.SubjectPrefix = "test-" + stream
	cfg.ConsumerName = consumer
	cfg.ConnectTimeout = 3 * time.Second
	// Short enough that a redelivery test finishes, long enough that a slow
	// handler is not redelivered underneath itself.
	cfg.AckWait = 2 * time.Second
	cfg.MaxDeliver = 5

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := natsmq.Connect(ctx, cfg, discardLogger())
	if err != nil {
		t.Skipf("no NATS broker at %s (run `make broker-up`): %v", url, err)
	}
	t.Cleanup(conn.Close)
	return conn
}

// uniqueName keeps concurrent tests off each other's streams.
func uniqueName(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano()%1_000_000)
}

func newEvent(t *testing.T, eventType, payload string) messaging.Event {
	t.Helper()
	return messaging.Event{
		ID:          uuid.Must(uuid.NewV7()),
		Type:        eventType,
		OccurredAt:  time.Now().UTC().Truncate(time.Second),
		TraceParent: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		Payload:     json.RawMessage(payload),
	}
}

// processorFunc adapts a function to natsmq.Processor.
type processorFunc func(ctx context.Context, event messaging.Event) error

func (f processorFunc) Process(ctx context.Context, e messaging.Event) error { return f(ctx, e) }

// TestAnEventSurvivesTheRoundTrip is the transport contract: everything the
// consumer needs must come back, because a field dropped in transit is a field
// no amount of consumer-side testing can recover.
func TestAnEventSurvivesTheRoundTrip(t *testing.T) {
	stream := uniqueName("ROUNDTRIP")
	conn := connect(t, stream, "roundtrip-consumer")
	ctx := context.Background()

	sub, err := natsmq.Subscribe(ctx, conn, discardLogger(), "user.created.v1")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	sent := newEvent(t, "user.created.v1", `{"email":"ada@example.com"}`)
	received := make(chan messaging.Event, 1)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = sub.Run(runCtx, processorFunc(func(_ context.Context, e messaging.Event) error {
			select {
			case received <- e:
			default:
			}
			return nil
		}))
	}()

	if err := natsmq.NewPublisher(conn).Publish(ctx, sent); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case got := <-received:
		if got.ID != sent.ID {
			t.Errorf("id = %s, want %s", got.ID, sent.ID)
		}
		if got.Type != sent.Type {
			t.Errorf("type = %q, want %q", got.Type, sent.Type)
		}
		// Without the traceparent the asynchronous work is an orphan span and
		// nobody can tell which request caused it.
		if got.TraceParent != sent.TraceParent {
			t.Errorf("traceparent = %q, want %q", got.TraceParent, sent.TraceParent)
		}
		if !got.OccurredAt.Equal(sent.OccurredAt) {
			t.Errorf("occurred_at = %s, want %s", got.OccurredAt, sent.OccurredAt)
		}
		if string(got.Payload) != string(sent.Payload) {
			t.Errorf("payload = %s, want %s", got.Payload, sent.Payload)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the published event never arrived")
	}

	cancel()
	<-done
}

// TestAnUnacknowledgedEventIsRedelivered is the at-least-once guarantee seen
// from the transport. A handler that fails must get the message again, or the
// retry logic above it is decoration.
func TestAnUnacknowledgedEventIsRedelivered(t *testing.T) {
	stream := uniqueName("REDELIVER")
	conn := connect(t, stream, "redeliver-consumer")
	ctx := context.Background()

	sub, err := natsmq.Subscribe(ctx, conn, discardLogger(), "user.created.v1")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	var (
		mu       sync.Mutex
		attempts int
	)
	twice := make(chan struct{})

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = sub.Run(runCtx, processorFunc(func(_ context.Context, _ messaging.Event) error {
			mu.Lock()
			attempts++
			n := attempts
			mu.Unlock()

			if n == 1 {
				return errors.New("the handler failed on the first delivery")
			}
			select {
			case twice <- struct{}{}:
			default:
			}
			return nil
		}))
	}()

	if err := natsmq.NewPublisher(conn).Publish(ctx, newEvent(t, "user.created.v1", `{"n":1}`)); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case <-twice:
	case <-time.After(30 * time.Second):
		mu.Lock()
		n := attempts
		mu.Unlock()
		t.Fatalf("the event was delivered %d times; a failed handler must get it again", n)
	}

	cancel()
	<-done
}

// TestAMessageWithNoEnvelopeIsNotRetriedForever covers the poison message. A
// message that cannot be decoded will not decode on the tenth attempt, and
// leaving it unacknowledged fills the redelivery queue with something no
// consumer can ever accept.
func TestAMessageWithNoEnvelopeIsNotRetriedForever(t *testing.T) {
	stream := uniqueName("POISON")
	conn := connect(t, stream, "poison-consumer")
	ctx := context.Background()

	sub, err := natsmq.Subscribe(ctx, conn, discardLogger(), "user.created.v1")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Publish with the NATS client directly so the message reaches the stream
	// without the envelope headers. Going through natsmq.Publisher could not
	// produce this message, and it is exactly the message a service in another
	// language, or an older version of this one, would send.
	url := os.Getenv("MESSAGING_NATS_URLS")
	if url == "" {
		url = "nats://localhost:4222"
	}
	raw, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connecting directly to NATS: %v", err)
	}
	defer raw.Close()
	if err := raw.Publish("test-"+stream+".user.created.v1", []byte(`{"not":"an envelope"}`)); err != nil {
		t.Fatalf("publishing a raw message: %v", err)
	}
	if err := raw.Flush(); err != nil {
		t.Fatalf("flushing: %v", err)
	}

	var (
		mu        sync.Mutex
		processed int
	)

	runCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = sub.Run(runCtx, processorFunc(func(_ context.Context, _ messaging.Event) error {
			mu.Lock()
			processed++
			mu.Unlock()
			return nil
		}))
	}()
	<-done

	mu.Lock()
	count := processed
	mu.Unlock()
	if count != 0 {
		t.Errorf("an undecodable message reached the processor %d times", count)
	}

	// "It never reached the processor" is true whether the message was taken
	// out of rotation or is being redelivered forever, so it does not
	// distinguish the two. The broker's own counters do: a terminated message
	// leaves nothing pending, while a naked one accumulates redeliveries until
	// MaxDeliver and holds a slot the whole time.
	info := consumerInfo(t, stream, "poison-consumer")
	if info.NumAckPending != 0 {
		t.Errorf("%d messages are still awaiting acknowledgement; the undecodable "+
			"message was left in rotation instead of being terminated", info.NumAckPending)
	}
	if info.NumRedelivered != 0 {
		t.Errorf("the undecodable message was redelivered %d times; redelivery "+
			"cannot make it decode", info.NumRedelivered)
	}
}

// TestSubscribingTwiceKeepsTheConsumerPosition is what makes a restart safe.
// An ephemeral consumer would forget where it was and either replay everything
// or skip whatever arrived while the worker was down.
func TestSubscribingTwiceKeepsTheConsumerPosition(t *testing.T) {
	stream := uniqueName("DURABLE")
	conn := connect(t, stream, "durable-consumer")
	ctx := context.Background()

	publisher := natsmq.NewPublisher(conn)
	first := newEvent(t, "user.created.v1", `{"n":1}`)
	if err := publisher.Publish(ctx, first); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	consume := func(timeout time.Duration) []uuid.UUID {
		sub, err := natsmq.Subscribe(ctx, conn, discardLogger(), "user.created.v1")
		if err != nil {
			t.Fatalf("Subscribe: %v", err)
		}

		var (
			mu   sync.Mutex
			seen []uuid.UUID
		)
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		_ = sub.Run(runCtx, processorFunc(func(_ context.Context, e messaging.Event) error {
			mu.Lock()
			seen = append(seen, e.ID)
			mu.Unlock()
			return nil
		}))
		mu.Lock()
		defer mu.Unlock()
		return seen
	}

	if got := consume(6 * time.Second); len(got) != 1 || got[0] != first.ID {
		t.Fatalf("the first run saw %v, want just %s", got, first.ID)
	}

	// A second subscription with the same durable name must not replay what
	// the first already acknowledged.
	second := newEvent(t, "user.created.v1", `{"n":2}`)
	if err := publisher.Publish(ctx, second); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	got := consume(6 * time.Second)
	for _, id := range got {
		if id == first.ID {
			t.Error("an acknowledged event was replayed after resubscribing; " +
				"the consumer is not durable")
		}
	}
	if len(got) != 1 || got[0] != second.ID {
		t.Errorf("the second run saw %v, want just %s", got, second.ID)
	}

	// Resubscribing within one process reuses whatever consumer already
	// exists, so the assertions above hold even for a non-durable consumer.
	// The property that actually matters — the position survives the process
	// dying — is only visible in the consumer JetStream persisted, so it is
	// asserted directly.
	assertConsumerIsDurable(t, stream, "durable-consumer")
}

// assertConsumerIsDurable inspects the consumer JetStream is holding.
//
// A named-but-ephemeral consumer is deleted after an inactivity threshold,
// which means a worker that is down for longer than that comes back and either
// replays the whole stream or skips everything it missed. Nothing observable
// inside a single test run distinguishes the two, so the broker is asked.
func assertConsumerIsDurable(t *testing.T, stream, consumer string) {
	t.Helper()

	info := consumerInfo(t, stream, consumer)

	if info.Config.Durable == "" {
		t.Errorf("consumer %s has no durable name; JetStream will delete it after "+
			"%s of inactivity and the worker will lose its position",
			consumer, info.Config.InactiveThreshold)
	}
	if info.Config.AckPolicy != jetstream.AckExplicitPolicy {
		t.Errorf("ack policy = %v, want explicit; anything else acknowledges messages "+
			"before the handler has committed", info.Config.AckPolicy)
	}
}

// consumerInfo asks the broker what it actually holds, rather than trusting
// what the code intended to create.
func consumerInfo(t *testing.T, stream, consumer string) *jetstream.ConsumerInfo {
	t.Helper()

	url := os.Getenv("MESSAGING_NATS_URLS")
	if url == "" {
		url = "nats://localhost:4222"
	}
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connecting to NATS: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := js.Consumer(ctx, stream, consumer)
	if err != nil {
		t.Fatalf("looking up consumer %s on stream %s: %v", consumer, stream, err)
	}
	info, err := c.Info(ctx)
	if err != nil {
		t.Fatalf("consumer info: %v", err)
	}
	return info
}
