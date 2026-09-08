package httpx

import (
	"net/http"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
)

// TestStatusByKindIsExhaustive fails when a Kind is added to apperr without a
// row in the mapping table. Without it, a new Kind would silently become a
// 500, and the omission would only surface in production.
func TestStatusByKindIsExhaustive(t *testing.T) {
	// The upper bound is discovered by walking until String() stops producing
	// a distinct name, so this test needs no maintenance when a Kind is added.
	for k := apperr.Kind(0); ; k++ {
		name := k.String()
		if name == "unknown" || name == "" {
			break
		}
		if _, ok := statusByKind[k]; !ok {
			t.Errorf("Kind %q (%d) has no HTTP status mapping", name, k)
		}
		if k > 64 {
			t.Fatal("Kind.String() never returned \"unknown\"; the sentinel is missing")
		}
	}
}

func TestStatusOf(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{apperr.Invalid("X", "x"), http.StatusBadRequest},
		{apperr.NotFound("X", "x"), http.StatusNotFound},
		{apperr.Unauthorized("X", "x"), http.StatusUnauthorized},
		{apperr.Forbidden("X", "x"), http.StatusForbidden},
		{apperr.Conflict("X", "x"), http.StatusConflict},
		{apperr.RateLimited("X", "x"), http.StatusTooManyRequests},
		{apperr.New(apperr.KindInternal, "X", "x"), http.StatusInternalServerError},
	}

	for _, c := range cases {
		if got := StatusOf(c.err); got != c.want {
			t.Errorf("StatusOf(%v) = %d, want %d", c.err, got, c.want)
		}
	}
}

func TestStatusOfUnclassifiedErrorIs500(t *testing.T) {
	if got := StatusOf(errStub{}); got != http.StatusInternalServerError {
		t.Fatalf("StatusOf(plain error) = %d, want 500", got)
	}
}

type errStub struct{}

func (errStub) Error() string { return "stub" }
