package validate_test

import (
	"testing"

	"github.com/example/go-enterprise/internal/apperr"
	"github.com/example/go-enterprise/internal/validate"
)

func TestViolationsAccumulate(t *testing.T) {
	// Reporting one field at a time turns a form submission into a guessing
	// game, so every failure must be collected before returning.
	var v validate.Violations
	v.Required("name", "")
	v.Email("email", "not-an-email")

	err := v.Err("VALIDATION_FAILED", "invalid request")
	if err == nil {
		t.Fatal("Err() = nil, want an error")
	}

	appErr := apperr.From(err)
	if appErr.Kind != apperr.KindInvalidArgument {
		t.Fatalf("Kind = %v, want KindInvalidArgument", appErr.Kind)
	}
	if len(appErr.Details) != 2 {
		t.Fatalf("Details = %v, want two entries", appErr.Details)
	}
}

func TestNoViolationsMeansNoError(t *testing.T) {
	var v validate.Violations
	v.Required("name", "Ada")

	if err := v.Err("VALIDATION_FAILED", "invalid request"); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
}

func TestRequiredRejectsWhitespaceOnly(t *testing.T) {
	var v validate.Violations
	if v.Required("name", "   \t\n ") {
		t.Fatal("whitespace-only value accepted")
	}
}

func TestEmail(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"user@example.com", true},
		{"user+tag@sub.example.co.uk", true},
		{"user@example", false},  // no dot in the domain
		{"user", false},          // no domain at all
		{"@example.com", false},  // no local part
		{"", false},              // empty
		{"Ada <a@b.com>", false}, // display-name form is not an address
		{"a@b.com ", false},      // trailing space would be silently accepted
	}

	for _, c := range cases {
		var v validate.Violations
		if got := v.Email("email", c.in); got != c.want {
			t.Errorf("Email(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestRuneLenCountsRunesNotBytes(t *testing.T) {
	// "世界" is 2 runes but 6 bytes. A byte-based check would reject it.
	var v validate.Violations
	if !v.RuneLen("name", "世界", 2, 2) {
		t.Fatalf("RuneLen rejected a 2-rune value: %v", v.Items())
	}
}

func TestRuneLenBoundaries(t *testing.T) {
	cases := []struct {
		in       string
		min, max int
		want     bool
	}{
		{"ab", 2, 4, true},
		{"abcd", 2, 4, true},
		{"a", 2, 4, false},
		{"abcde", 2, 4, false},
	}

	for _, c := range cases {
		var v validate.Violations
		if got := v.RuneLen("f", c.in, c.min, c.max); got != c.want {
			t.Errorf("RuneLen(%q, %d, %d) = %v, want %v", c.in, c.min, c.max, got, c.want)
		}
	}
}

func TestOneOf(t *testing.T) {
	var v validate.Violations
	if !v.OneOf("role", "admin", "user", "admin") {
		t.Fatal("OneOf rejected a listed value")
	}
	if v.OneOf("role", "root", "user", "admin") {
		t.Fatal("OneOf accepted an unlisted value")
	}
}

func TestIntRange(t *testing.T) {
	var v validate.Violations
	if !v.IntRange("page", 1, 1, 100) {
		t.Fatal("IntRange rejected the lower bound")
	}
	if v.IntRange("page", 101, 1, 100) {
		t.Fatal("IntRange accepted a value above the upper bound")
	}
}

func TestNormalizeEmail(t *testing.T) {
	// Case-insensitive comparison is what makes the unique index meaningful:
	// without normalisation, "A@b.com" and "a@b.com" become two accounts.
	cases := map[string]string{
		"  User@Example.COM  ": "user@example.com",
		"user@example.com":     "user@example.com",
	}

	for in, want := range cases {
		if got := validate.NormalizeEmail(in); got != want {
			t.Errorf("NormalizeEmail(%q) = %q, want %q", in, got, want)
		}
	}
}
