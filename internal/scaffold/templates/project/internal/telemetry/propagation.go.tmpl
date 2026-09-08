package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// ContextWithTraceParent returns a context whose active span context is the
// remote parent described by a W3C traceparent header.
//
// This is the other half of the outbox's traceparent column. The relay picks
// an event's stored header up here so that its publish span, and in turn the
// consumer's span, land in the trace of the HTTP request that created the
// event. Without it every asynchronous effect appears as an unexplained root
// trace, and the one question worth asking of a background system — what
// caused this? — has no answer.
//
// It is deliberately tolerant. A header that is absent, truncated, or carrying
// an all-zero id yields the context unchanged, so the caller's span becomes a
// root. That is honest: an obviously separate trace is far better than one
// stitched onto a fabricated parent, which would look joined and lead
// investigators to a trace id that never existed.
func ContextWithTraceParent(ctx context.Context, traceparent string) context.Context {
	if traceparent == "" {
		return ctx
	}

	extracted := Propagator().Extract(ctx, singleHeader{
		key: "traceparent", value: traceparent,
	})

	// The W3C propagator already rejects these, so today this check never
	// fires; a mutation removing it survives the tests. It stays because the
	// guarantee belongs to this function, not to whichever propagator
	// Propagator() happens to compose. B3 and Jaeger extractors have been
	// looser about zero ids, and adding one for a legacy peer must not
	// silently turn "unparseable header" into "everything is one trace".
	if !trace.SpanContextFromContext(extracted).IsValid() {
		return ctx
	}
	return extracted
}

// TraceParentFromContext renders the active span as a W3C traceparent, or "" if
// there is no valid span — which is the normal state when tracing is disabled.
//
// Returning "" rather than a zero-id header matters: a header of all zeroes is
// syntactically well formed, so it would be stored in the outbox and extracted
// again, quietly collapsing every event in the system onto one trace id.
func TraceParentFromContext(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return ""
	}
	return "00-" + sc.TraceID().String() + "-" + sc.SpanID().String() +
		"-" + sc.TraceFlags().String()
}

// singleHeader is a TextMapCarrier over one header, so that extraction does not
// require allocating an http.Header for a value we already hold.
type singleHeader struct{ key, value string }

func (h singleHeader) Get(key string) string {
	if key == h.key {
		return h.value
	}
	return ""
}

func (h singleHeader) Set(string, string) {}

func (h singleHeader) Keys() []string { return []string{h.key} }
