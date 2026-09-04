package apperr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
)

func TestZeroKindIsInternal(t *testing.T) {
	// The zero value must be the most conservative classification: an error
	// constructed without thought must not accidentally return 404 or leak.
	var k apperr.Kind
	if k != apperr.KindInternal {
		t.Fatalf("zero Kind = %v, want KindInternal", k)
	}

	var e apperr.Error
	if e.Kind != apperr.KindInternal {
		t.Fatalf("zero Error.Kind = %v, want KindInternal", e.Kind)
	}
}

func TestKindOfWalksTheChain(t *testing.T) {
	base := apperr.NotFound("USER_NOT_FOUND", "user not found")
	wrapped := fmt.Errorf("service layer: %w", fmt.Errorf("repo layer: %w", base))

	if got := apperr.KindOf(wrapped); got != apperr.KindNotFound {
		t.Fatalf("KindOf through two wraps = %v, want KindNotFound", got)
	}
	if !apperr.Is(wrapped, apperr.KindNotFound) {
		t.Fatal("Is() failed through two wraps")
	}
}

func TestKindOfUnclassifiedIsInternal(t *testing.T) {
	if got := apperr.KindOf(errors.New("boom")); got != apperr.KindInternal {
		t.Fatalf("KindOf(plain error) = %v, want KindInternal", got)
	}
}

func TestUnwrapExposesCause(t *testing.T) {
	sentinel := errors.New("sentinel")
	err := apperr.Wrap(sentinel, apperr.KindConflict, "CONFLICT", "conflict")

	if !errors.Is(err, sentinel) {
		t.Fatal("errors.Is could not reach the cause")
	}
}

func TestInternalHidesTheCause(t *testing.T) {
	// The Message is what reaches the client. It must never carry the cause.
	err := apperr.Internal(errors.New("pq: relation \"users\" does not exist"))

	if err.Message != "internal error" {
		t.Fatalf("Message = %q, want %q", err.Message, "internal error")
	}
	if err.Code != apperr.CodeInternal {
		t.Fatalf("Code = %q, want %q", err.Code, apperr.CodeInternal)
	}
	if err.Details != nil {
		t.Fatalf("Details = %v, want nil", err.Details)
	}
}

func TestFromAlwaysReturnsAnError(t *testing.T) {
	got := apperr.From(errors.New("boom"))
	if got == nil {
		t.Fatal("From(plain error) = nil, want a usable *Error")
	}
	if got.Kind != apperr.KindInternal || got.Code != apperr.CodeInternal {
		t.Fatalf("From(plain error) = %+v, want internal classification", got)
	}
}

func TestWithDetailsDoesNotMutateTheOriginal(t *testing.T) {
	base := apperr.Invalid("VALIDATION_FAILED", "invalid request")
	derived := base.WithDetails(apperr.Detail{Field: "email", Reason: "required"})

	if len(base.Details) != 0 {
		t.Fatalf("original mutated: %v", base.Details)
	}
	if len(derived.Details) != 1 {
		t.Fatalf("derived Details = %v, want one entry", derived.Details)
	}
}

func TestKindStringIsStable(t *testing.T) {
	// These strings appear in logs and dashboards; renaming one silently
	// breaks queries, so the mapping is pinned.
	cases := map[apperr.Kind]string{
		apperr.KindInternal:        "internal",
		apperr.KindInvalidArgument: "invalid_argument",
		apperr.KindNotFound:        "not_found",
		apperr.KindUnauthorized:    "unauthorized",
		apperr.KindForbidden:       "forbidden",
		apperr.KindConflict:        "conflict",
		apperr.KindRateLimited:     "rate_limited",
	}

	for kind, want := range cases {
		if got := kind.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", kind, got, want)
		}
	}
}
