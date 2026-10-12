// Package messaging defines the event envelope and the publishing interface
// that business code sees.
//
// Nothing in this file knows what a broker is. That is the whole point: a
// service package can publish an event without importing a client library, and
// the choice between NATS, a database outbox and an in-memory slice becomes a
// wiring decision in main rather than a rewrite. ADR-0009 states the rule and
// .golangci.yml enforces it — business packages are denied the NATS import
// outright, because a rule that lives only in a document is a rule that will
// be broken.
package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Event is one thing that happened, described in a form no broker dictates.
//
// There is no subject, partition, offset or sequence here. Those are transport
// details, and a consumer that learns them is a consumer that cannot be moved.
type Event struct {
	// ID is the idempotency key. It is a UUIDv7 so that the ordering of the
	// keys matches the ordering of the events, which makes the outbox index
	// useful and makes a dump of the table readable.
	ID uuid.UUID
	// Type is "{aggregate}.{action}.v{n}", for example "user.created.v1". The
	// version is part of the type rather than a separate field because a
	// consumer subscribes to a contract, not to a name plus a number it has to
	// remember to check.
	Type string
	// OccurredAt is when the fact became true, not when it was delivered.
	OccurredAt time.Time
	// TraceParent is the W3C traceparent of the request that caused the event,
	// so the asynchronous work shows up on the same trace as the HTTP call
	// that triggered it. Empty when there was no active trace.
	TraceParent string
	// Payload is the event body. It is deliberately raw: this package must not
	// grow a switch over every event type in the system.
	Payload json.RawMessage
	// ReplayGeneration counts how many times a dead-letter replay put this
	// event back into the outbox; zero for its first delivery. It is not part
	// of the event's identity: consumers deduplicate on ID, which is what
	// makes a replay safe. A broker that deduplicates publishes must key on
	// ID and ReplayGeneration together, or a replay inside its window is
	// dropped as a copy of the previous delivery.
	ReplayGeneration int32
}

// NewEvent builds an event with a fresh ID and the current time.
//
// The payload is marshalled here rather than by the caller so that a value
// which cannot be encoded fails at the publish site, where there is still a
// request to return an error to, instead of inside the relay hours later.
func NewEvent(eventType string, payload any) (Event, error) {
	if err := ValidateType(eventType); err != nil {
		return Event{}, err
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("marshal payload for %s: %w", eventType, err)
	}

	return Event{
		ID:         uuid.Must(uuid.NewV7()),
		Type:       eventType,
		OccurredAt: time.Now().UTC(),
		Payload:    encoded,
	}, nil
}

// ValidateType rejects a type that does not carry a version.
//
// An unversioned event type is the beginning of every incompatible change that
// ever broke a consumer silently: with no version there is nowhere to put the
// warning, so the change simply arrives. Checking the shape here means the
// mistake is a test failure rather than a production incident.
func ValidateType(eventType string) error {
	parts := strings.Split(eventType, ".")
	if len(parts) < 3 {
		return fmt.Errorf(
			"event type %q must be {aggregate}.{action}.v{n}, for example user.created.v1",
			eventType)
	}

	version := parts[len(parts)-1]
	if len(version) < 2 || version[0] != 'v' {
		return fmt.Errorf("event type %q must end in a version such as v1", eventType)
	}
	for _, r := range version[1:] {
		if r < '0' || r > '9' {
			return fmt.Errorf("event type %q must end in a version such as v1", eventType)
		}
	}

	for _, part := range parts[:len(parts)-1] {
		if part == "" {
			return fmt.Errorf("event type %q has an empty segment", eventType)
		}
	}
	return nil
}

// Publisher accepts events for delivery.
//
// The contract is deliberately weak: a successful return means the event has
// been durably accepted, not that anyone has received it. Delivery is
// at-least-once and asynchronous, and any code that reads Publish as "it has
// been handled" is code that will be wrong under load.
type Publisher interface {
	Publish(ctx context.Context, event Event) error
}

// Handler processes one event.
//
// An error means the event should be retried. Returning nil for a message the
// handler chose to ignore is correct and expected: "not for me" is a success,
// because a retry would not change it.
type Handler interface {
	Handle(ctx context.Context, event Event) error
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(ctx context.Context, event Event) error

// Handle calls f.
func (f HandlerFunc) Handle(ctx context.Context, event Event) error { return f(ctx, event) }
