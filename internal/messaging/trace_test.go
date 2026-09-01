package messaging_test

import (
	"context"
	"testing"

	"github.com/example/go-enterprise/internal/messaging"
)

const goodTraceParent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

// TestValidTraceParent covers the W3C format. The cases that matter are the
// ones a lenient check would wave through: an all-zero id means "no trace" and
// uppercase hex is invalid, and either of them stored as if it were real
// produces traces that appear to be joined but are not.
func TestValidTraceParent(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		traceparent string
		want        bool
	}{
		"a well-formed header":             {goodTraceParent, true},
		"sampled flag off":                 {"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00", true},
		"a future version":                 {"01-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", true},
		"empty":                            {"", false},
		"version ff is reserved":           {"ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", false},
		"an all-zero trace id":             {"00-00000000000000000000000000000000-00f067aa0ba902b7-01", false},
		"an all-zero parent id":            {"00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01", false},
		"uppercase hex":                    {"00-4BF92F3577B34DA6A3CE929D0E0E4736-00f067aa0ba902b7-01", false},
		"a trace id one digit short":       {"00-4bf92f3577b34da6a3ce929d0e473-00f067aa0ba902b7-01", false},
		"a trace id one digit long":        {"00-4bf92f3577b34da6a3ce929d0e0e47366-00f067aa0ba902b7-01", false},
		"too few fields":                   {"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7", false},
		"too many fields":                  {goodTraceParent + "-extra", false},
		"non-hex characters":               {"00-4bf92f3577b34da6a3ce929d0e0e473g-00f067aa0ba902b7-01", false},
		"whitespace around a valid header": {" " + goodTraceParent + " ", false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := messaging.ValidTraceParent(tc.traceparent); got != tc.want {
				t.Errorf("ValidTraceParent(%q) = %v, want %v", tc.traceparent, got, tc.want)
			}
		})
	}
}

// TestAValidTraceParentSurvivesTheContext is the round trip the whole feature
// depends on.
func TestAValidTraceParentSurvivesTheContext(t *testing.T) {
	t.Parallel()

	ctx := messaging.ContextWithTraceParent(context.Background(), goodTraceParent)
	if got := messaging.TraceParentFromContext(ctx); got != goodTraceParent {
		t.Errorf("TraceParentFromContext = %q, want %q", got, goodTraceParent)
	}
}

// TestAMalformedTraceParentIsDroppedRatherThanStored covers the decision that
// a bad header is worse than no header. Storing an attacker-supplied or
// truncated value would put fabricated parent ids on real traces.
func TestAMalformedTraceParentIsDroppedRatherThanStored(t *testing.T) {
	t.Parallel()

	ctx := messaging.ContextWithTraceParent(context.Background(), "not-a-traceparent")
	if got := messaging.TraceParentFromContext(ctx); got != "" {
		t.Errorf("TraceParentFromContext = %q, want the malformed header to have been dropped", got)
	}
}

// TestABadTraceParentDoesNotOverwriteAGoodOne is the case a naive
// implementation gets wrong: validating the input but still calling
// WithValue would replace a valid trace with an empty string.
func TestABadTraceParentDoesNotOverwriteAGoodOne(t *testing.T) {
	t.Parallel()

	ctx := messaging.ContextWithTraceParent(context.Background(), goodTraceParent)
	ctx = messaging.ContextWithTraceParent(ctx, "garbage")

	if got := messaging.TraceParentFromContext(ctx); got != goodTraceParent {
		t.Errorf("TraceParentFromContext = %q, want the original %q to have survived",
			got, goodTraceParent)
	}
}

// TestAContextWithNoTraceParentReturnsEmpty checks the lookup does not panic
// on the common path, since most background work has no trace at all.
func TestAContextWithNoTraceParentReturnsEmpty(t *testing.T) {
	t.Parallel()

	if got := messaging.TraceParentFromContext(context.Background()); got != "" {
		t.Errorf("TraceParentFromContext on a bare context = %q, want empty", got)
	}
}
