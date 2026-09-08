package messaging

import (
	"context"
	"log/slog"
	"sync"
)

// MemoryPublisher collects events in memory.
//
// It exists for tests, and it deliberately keeps the events rather than
// counting them: an assertion that one event was published says much less than
// an assertion about which event, and a test that cannot see the payload
// cannot notice when a password ends up in it.
type MemoryPublisher struct {
	mu     sync.Mutex
	events []Event
	err    error
}

// NewMemoryPublisher returns an empty publisher.
func NewMemoryPublisher() *MemoryPublisher { return &MemoryPublisher{} }

// FailWith makes every subsequent Publish return err.
//
// Publishing is one of the few operations a service performs that can fail for
// reasons entirely outside its own logic, so the failure path needs to be
// reachable from a test without breaking anything real.
func (p *MemoryPublisher) FailWith(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.err = err
}

// Publish records the event.
func (p *MemoryPublisher) Publish(_ context.Context, event Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return p.err
	}
	p.events = append(p.events, event)
	return nil
}

// Events returns a copy of everything published so far.
func (p *MemoryPublisher) Events() []Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Event(nil), p.events...)
}

// EventsOfType returns the published events with the given type.
func (p *MemoryPublisher) EventsOfType(eventType string) []Event {
	var matched []Event
	for _, e := range p.Events() {
		if e.Type == eventType {
			matched = append(matched, e)
		}
	}
	return matched
}

// Reset drops the recorded events.
func (p *MemoryPublisher) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = nil
}

// NoopPublisher accepts events and drops them.
//
// This is what runs when messaging is disabled, which is the default. It logs
// at debug rather than staying silent: a service that was expected to emit
// events and emits nothing should be diagnosable without a code change, and
// the alternative — no record at all — makes "is the worker deployed?" an
// unanswerable question.
type NoopPublisher struct{ logger *slog.Logger }

// NewNoopPublisher returns a publisher that discards events.
func NewNoopPublisher(logger *slog.Logger) *NoopPublisher {
	return &NoopPublisher{logger: logger}
}

// Publish discards the event.
func (p *NoopPublisher) Publish(ctx context.Context, event Event) error {
	if p.logger != nil {
		p.logger.DebugContext(ctx, "event discarded: messaging is disabled",
			slog.String("event_type", event.Type),
			slog.String("event_id", event.ID.String()))
	}
	return nil
}
