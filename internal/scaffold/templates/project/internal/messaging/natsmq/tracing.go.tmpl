package natsmq

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/telemetry"
)

// tracerName identifies this instrumentation in the trace backend.
const tracerName = "github.com/namezzy/goscaffoldforlevi/internal/messaging/natsmq"

// startPublishSpan opens the producer span for one event.
//
// The span is parented on the event's stored traceparent, which the outbox
// carried over from the request that created the event. Parenting it on the
// relay's own loop instead would be easy and useless: every publish would hang
// off a long-running background trace, and the question that matters — which
// request caused this message — would have no answer.
func startPublishSpan(
	ctx context.Context, tp telemetry.TracerProvider, event messaging.Event,
) (context.Context, trace.Span) {
	ctx = telemetry.ContextWithTraceParent(ctx, event.TraceParent)

	// Named by event type, not by event id. The id is unique per message, and
	// a span name per message makes the backend's "slowest operations" view a
	// list of every message ever sent.
	return tp.Tracer(tracerName).Start(ctx, "publish "+event.Type,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.operation", "publish"),
			attribute.String("messaging.destination.name", event.Type),
		),
	)
}

// startConsumeSpan opens the consumer span for one delivered message.
func startConsumeSpan(
	ctx context.Context, tp telemetry.TracerProvider, event messaging.Event,
) (context.Context, trace.Span) {
	ctx = telemetry.ContextWithTraceParent(ctx, event.TraceParent)

	return tp.Tracer(tracerName).Start(ctx, "consume "+event.Type,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.operation", "process"),
			attribute.String("messaging.destination.name", event.Type),
		),
	)
}

// traceParentForMessage decides which traceparent to stamp on the outgoing
// message: the publish span's own, or the header the outbox stored.
//
// Preferring the live span is what makes the consumer a child of the publish
// rather than a sibling of it. Both would sit in the same trace, so the
// difference is invisible in a trace count and obvious in a trace: siblings
// say two things happened, a chain says one caused the other, and causality is
// the whole reason to correlate a broker at all.
//
// Falling back to the stored header is what keeps tracing optional. With
// tracing disabled there is no live span, and this returns the header that
// Phase 4 already propagated — so the correlation the outbox provides survives
// exactly as it did before, rather than being replaced by nothing.
func traceParentForMessage(ctx context.Context, stored string) string {
	if live := telemetry.TraceParentFromContext(ctx); live != "" {
		return live
	}
	return stored
}
