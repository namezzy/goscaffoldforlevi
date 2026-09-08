package messaging

import (
	"context"
	"strings"
)

// traceParentKey carries the W3C traceparent of the current request.
//
// The value is a string rather than an OpenTelemetry span context on purpose:
// a business package that imports the tracing SDK to publish an event has
// coupled its domain logic to its observability stack, and .golangci.yml
// forbids it. Actual instrumentation arrives in Phase 5 and will set this from
// the active span; until then the HTTP layer forwards whatever the caller sent,
// which is enough for the field to be carried end to end.
type traceParentKey struct{}

// ContextWithTraceParent attaches a W3C traceparent to ctx.
//
// A malformed value is dropped rather than stored. A traceparent is copied
// into every event a request produces, and a broken one is worse than none:
// it produces traces that look joined but are not.
func ContextWithTraceParent(ctx context.Context, traceparent string) context.Context {
	if !ValidTraceParent(traceparent) {
		return ctx
	}
	return context.WithValue(ctx, traceParentKey{}, traceparent)
}

// TraceParentFromContext returns the traceparent, or "" when there is none.
func TraceParentFromContext(ctx context.Context) string {
	value, _ := ctx.Value(traceParentKey{}).(string)
	return value
}

// ValidTraceParent reports whether s is a well-formed W3C traceparent.
//
// The format is version-trace_id-parent_id-flags, all lowercase hex, with an
// all-zero trace or parent id being invalid by the specification.
func ValidTraceParent(s string) bool {
	parts := strings.Split(s, "-")
	if len(parts) != 4 {
		return false
	}

	lengths := [4]int{2, 32, 16, 2}
	for i, part := range parts {
		if len(part) != lengths[i] || !isLowerHex(part) {
			return false
		}
	}

	// Version ff is reserved, and an all-zero id means "no trace".
	if parts[0] == "ff" || isAllZero(parts[1]) || isAllZero(parts[2]) {
		return false
	}
	return true
}

func isLowerHex(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f':
		default:
			return false
		}
	}
	return true
}

func isAllZero(s string) bool {
	for _, r := range s {
		if r != '0' {
			return false
		}
	}
	return true
}
