package natsmq

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/telemetry"
)

func recordingProvider(t *testing.T) (telemetry.TracerProvider, *tracetest.SpanRecorder) {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return tp, rec
}

const requestTrace = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

func tracedEvent() messaging.Event {
	return messaging.Event{Type: "user.created.v1", TraceParent: requestTrace}
}

// §5.7. The publish span must land in the trace of the request that produced
// the event. Starting it from the relay's own context would put every publish
// in a background trace that runs for the life of the process, where the only
// question worth asking — which request caused this message — cannot be asked.
func TestThePublishSpanJoinsTheProducingRequestsTrace(t *testing.T) {
	tp, rec := recordingProvider(t)

	_, span := startPublishSpan(context.Background(), tp, tracedEvent())
	span.End()

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	if got, want := spans[0].SpanContext().TraceID().String(),
		"4bf92f3577b34da6a3ce929d0e0e4736"; got != want {
		t.Errorf("trace id = %q, want the request's %q", got, want)
	}
	if got, want := spans[0].Parent().SpanID().String(), "00f067aa0ba902b7"; got != want {
		t.Errorf("parent span id = %q, want %q", got, want)
	}
}

// The consumer span must chain to the publish span, not merely share its
// trace. Two siblings say that two things happened; a chain says that one
// caused the other, and causality is the only reason to correlate a broker.
func TestTheConsumerSpanIsAChildOfThePublishSpan(t *testing.T) {
	tp, rec := recordingProvider(t)

	event := tracedEvent()
	publishCtx, publishSpan := startPublishSpan(context.Background(), tp, event)

	// What Publish stamps on the wire is what the consumer reads back.
	event.TraceParent = traceParentForMessage(publishCtx, event.TraceParent)
	publishSpan.End()

	_, consumeSpan := startConsumeSpan(context.Background(), tp, event)
	consumeSpan.End()

	spans := rec.Ended()
	if len(spans) != 2 {
		t.Fatalf("recorded %d spans, want 2", len(spans))
	}
	published, consumed := spans[0], spans[1]

	if consumed.Parent().SpanID() != published.SpanContext().SpanID() {
		t.Errorf("consumer parent = %v, want the publish span %v",
			consumed.Parent().SpanID(), published.SpanContext().SpanID())
	}
	if consumed.SpanContext().TraceID() != published.SpanContext().TraceID() {
		t.Errorf("consumer trace = %v, want %v",
			consumed.SpanContext().TraceID(), published.SpanContext().TraceID())
	}
}

// With tracing disabled there is no live span, and the message must still
// carry the header the outbox stored. Replacing it with nothing would silently
// remove the request correlation Phase 4 already provides, so turning tracing
// on for the first time would look like a feature and turning it back off
// would be a regression nobody connected to the flag.
func TestWithoutTracingTheStoredTraceParentIsStillStamped(t *testing.T) {
	ctx, span := telemetry.NoopTracerProvider().Tracer("t").
		Start(context.Background(), "publish")
	defer span.End()

	if got := traceParentForMessage(ctx, requestTrace); got != requestTrace {
		t.Errorf("stamped %q, want the stored %q", got, requestTrace)
	}
}

// An event with no stored traceparent and no live span must not gain one.
func TestAnUntracedEventStaysUntraced(t *testing.T) {
	if got := traceParentForMessage(context.Background(), ""); got != "" {
		t.Errorf("stamped %q on an untraced event, want empty", got)
	}
}

// Span names must be bounded by the event catalogue. Naming a span after the
// event id gives the backend one name per message, which turns every
// aggregate view — slowest operations, error rates by span — into a list of
// individual messages and makes the trace backend's own indexes unbounded.
func TestSpanNamesAreBoundedByEventType(t *testing.T) {
	tp, rec := recordingProvider(t)

	for i := 0; i < 3; i++ {
		event := tracedEvent()
		event.ID = messaging.Event{}.ID
		_, publish := startPublishSpan(context.Background(), tp, event)
		publish.End()
		_, consume := startConsumeSpan(context.Background(), tp, event)
		consume.End()
	}

	names := map[string]bool{}
	for _, s := range rec.Ended() {
		names[s.Name()] = true
	}
	want := map[string]bool{
		"publish user.created.v1": true,
		"consume user.created.v1": true,
	}
	if len(names) != len(want) {
		t.Fatalf("six messages produced %d distinct span names: %v", len(names), names)
	}
	for name := range want {
		if !names[name] {
			t.Errorf("missing span name %q, got %v", name, names)
		}
	}
}

// No payload may reach a span. Event payloads are the domain objects
// themselves — a user.created payload is a person's email address — and a
// trace backend is read by everyone who can open a dashboard.
func TestNoPayloadReachesTheSpan(t *testing.T) {
	tp, rec := recordingProvider(t)

	event := tracedEvent()
	event.Payload = []byte(`{"email":"victim@example.com"}`)

	_, publish := startPublishSpan(context.Background(), tp, event)
	publish.End()
	_, consume := startConsumeSpan(context.Background(), tp, event)
	consume.End()

	for _, s := range rec.Ended() {
		haystack := s.Name()
		for _, kv := range s.Attributes() {
			haystack += "\x00" + kv.Value.String()
		}
		if contains(haystack, "victim@example.com") {
			t.Errorf("span %q carries the payload: %s", s.Name(), haystack)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		func() bool {
			for i := 0; i+len(needle) <= len(haystack); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		}()
}
