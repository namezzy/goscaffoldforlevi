package messaging_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/example/go-enterprise/internal/messaging"
)

// TestAnEventTypeMustCarryAVersion protects the compatibility rule. An
// unversioned type gives a breaking change nowhere to announce itself, so the
// change simply arrives at consumers and they break quietly.
func TestAnEventTypeMustCarryAVersion(t *testing.T) {
	t.Parallel()

	accepted := []string{
		"user.created.v1",
		"user.created.v2",
		"order.line.added.v1", // extra segments are fine
		"user.created.v10",
	}
	for _, eventType := range accepted {
		t.Run("accepts "+eventType, func(t *testing.T) {
			t.Parallel()
			if err := messaging.ValidateType(eventType); err != nil {
				t.Errorf("ValidateType(%q) = %v, want it accepted", eventType, err)
			}
		})
	}

	rejected := map[string]string{
		"no version":             "user.created",
		"version is not a v":     "user.created.1",
		"version has no digits":  "user.created.v",
		"version is not numeric": "user.created.vNext",
		"empty aggregate":        ".created.v1",
		"empty action":           "user..v1",
		"a single word":          "created",
		"empty string":           "",
	}
	for name, eventType := range rejected {
		t.Run("rejects "+name, func(t *testing.T) {
			t.Parallel()
			if err := messaging.ValidateType(eventType); err == nil {
				t.Errorf("ValidateType(%q) accepted an invalid type", eventType)
			}
		})
	}
}

// TestNewEventRejectsAPayloadItCannotEncode checks that the failure happens at
// the publish site. If the payload were marshalled later by the relay, the
// error would surface in a background process hours after the request that
// caused it had returned 201.
func TestNewEventRejectsAPayloadItCannotEncode(t *testing.T) {
	t.Parallel()

	_, err := messaging.NewEvent("user.created.v1", math.Inf(1))
	if err == nil {
		t.Fatal("NewEvent accepted a payload that cannot be marshalled to JSON")
	}
	if !strings.Contains(err.Error(), "user.created.v1") {
		t.Errorf("error %q does not name the event type, so a log line would not identify it", err)
	}
}

// TestNewEventStampsAnIdentityOnEveryEvent covers the two fields the whole
// design rests on: a unique id for deduplication, and a timestamp of the fact
// rather than of its delivery.
func TestNewEventStampsAnIdentityOnEveryEvent(t *testing.T) {
	t.Parallel()

	first, err := messaging.NewEvent("user.created.v1", map[string]string{"email": "a@example.com"})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	second, err := messaging.NewEvent("user.created.v1", map[string]string{"email": "a@example.com"})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}

	// Two identical facts are still two events. Sharing an id would make the
	// consumer's deduplication silently drop the second one.
	if first.ID == second.ID {
		t.Error("two events were given the same id, which deduplication would treat as a duplicate")
	}
	if first.OccurredAt.IsZero() {
		t.Error("OccurredAt was not set")
	}
	if first.OccurredAt.Location() != time.UTC {
		t.Errorf("OccurredAt is in %s; timestamps crossing a process boundary must be UTC",
			first.OccurredAt.Location())
	}

	var decoded map[string]string
	if err := json.Unmarshal(first.Payload, &decoded); err != nil {
		t.Fatalf("the payload is not valid JSON: %v", err)
	}
	if decoded["email"] != "a@example.com" {
		t.Errorf("payload = %v, want the email preserved", decoded)
	}
}

// TestEventIDsSortInTheOrderTheEventsHappened checks the UUIDv7 property the
// outbox index depends on. Random ids would still be unique but would scatter
// inserts across the index and make a dump of the table unreadable.
func TestEventIDsSortInTheOrderTheEventsHappened(t *testing.T) {
	t.Parallel()

	const count = 20
	previous := ""
	for i := 0; i < count; i++ {
		event, err := messaging.NewEvent("user.created.v1", i)
		if err != nil {
			t.Fatalf("NewEvent: %v", err)
		}
		current := event.ID.String()
		if previous != "" && current <= previous {
			t.Fatalf("event %d has id %s which does not sort after %s", i, current, previous)
		}
		previous = current
	}
}

// TestTheMemoryPublisherRecordsWhatItWasGiven keeps the test double honest.
// Every test that asserts "an event was published" is only as trustworthy as
// this.
func TestTheMemoryPublisherRecordsWhatItWasGiven(t *testing.T) {
	t.Parallel()

	publisher := messaging.NewMemoryPublisher()
	event := testEvent(t)

	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	published := publisher.Events()
	if len(published) != 1 || published[0].ID != event.ID {
		t.Fatalf("Events() = %v, want the one published event", published)
	}

	// The returned slice must be a copy. If it aliased the internal one, a
	// test that ranged over it while the publisher was still in use would race.
	published[0].Type = "mutated"
	if publisher.Events()[0].Type == "mutated" {
		t.Error("Events() exposes the internal slice; a caller can corrupt the publisher's state")
	}
}

// TestTheNoopPublisherAcceptsAndDiscards documents the disabled-messaging path.
// It must not error, or turning messaging off would fail every write that
// publishes an event.
func TestTheNoopPublisherAcceptsAndDiscards(t *testing.T) {
	t.Parallel()

	publisher := messaging.NewNoopPublisher(discardLogger())
	if err := publisher.Publish(context.Background(), testEvent(t)); err != nil {
		t.Errorf("the no-op publisher returned %v; with messaging disabled this would fail writes", err)
	}
}

// TestAPermanentErrorIsRecognisableThroughWrapping matters because handlers
// wrap errors as they return them. If errors.As stopped seeing through the
// wrapping, a poison message would be retried to exhaustion instead of being
// dead-lettered at once.
func TestAPermanentErrorIsRecognisableThroughWrapping(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("handling event: %w",
		fmt.Errorf("decode payload: %w", messaging.Permanent(errors.New("bad json"))))

	if !errors.Is(wrapped, messaging.ErrPermanent) {
		t.Fatal("a permanent error wrapped twice is no longer recognisable, so it would be retried")
	}
	if !strings.Contains(wrapped.Error(), "bad json") {
		t.Errorf("error %q lost the underlying cause", wrapped)
	}

	// An ordinary error must not be mistaken for a permanent one, or a broker
	// outage would dead-letter everything instead of retrying.
	if errors.Is(errors.New("connection refused"), messaging.ErrPermanent) {
		t.Error("an ordinary error was classified as permanent")
	}
}
