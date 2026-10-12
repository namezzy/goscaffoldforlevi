package validate_test

import (
	"strings"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/validate"
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

func TestUsername(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"ada", true},
		{"ada_lovelace", true},
		{"ada-lovelace-2", true},
		{"a1", false},                    // too short
		{strings.Repeat("a", 33), false}, // too long
		{"1ada", false},                  // must start with a letter
		{"_ada", false},                  // must start with a letter
		{"ada lovelace", false},          // no spaces
		{"ada@lovelace", false},          // must not be confusable with an email
		{"", false},
	}

	for _, c := range cases {
		var v validate.Violations
		if got := v.Username("username", c.in); got != c.want {
			t.Errorf("Username(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNormalizeUsername(t *testing.T) {
	cases := map[string]string{
		"  Ada_Lovelace  ": "ada_lovelace",
		"ada_lovelace":     "ada_lovelace",
	}

	for in, want := range cases {
		if got := validate.NormalizeUsername(in); got != want {
			t.Errorf("NormalizeUsername(%q) = %q, want %q", in, got, want)
		}
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

// TestNoControlCharsRejectsWhatPostgreSQLCannotStore covers the case that
// reached production behaviour as a 500.
//
// A name containing U+0000 passed every rule — it is one rune, so the length
// check was satisfied — and PostgreSQL then refused it with SQLSTATE 22021,
// which surfaced to the client as an internal error. Text a user typed is not
// an internal error.
func TestNoControlCharsRejectsWhatPostgreSQLCannotStore(t *testing.T) {
	t.Parallel()

	rejected := map[string]string{
		"nul":              "A\x00B",
		"leading nul":      "\x00Name",
		"bell":             "A\aB",
		"escape":           "A\x1bB",
		"newline":          "A\nB",
		"tab":              "A\tB",
		"carriage return":  "A\rB",
		"delete":           "A\x7fB",
		"line separator":   "A\u2028B",
		"rtl override":     "admin\u202egnp.exe",
		"zero width space": "A\u200bB",
	}
	for name, input := range rejected {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var v validate.Violations
			if v.NoControlChars("name", input) {
				t.Fatalf("%q was accepted", input)
			}
			if !v.Any() {
				t.Fatal("no violation was recorded")
			}
			if got := v.Items()[0].Reason; got != "invalid_characters" {
				t.Errorf("reason = %q, want %q", got, "invalid_characters")
			}
		})
	}

	// The rule must not reject text people legitimately have in their names,
	// or it trades a 500 for a wrongly rejected user.
	accepted := []string{
		"Ada Lovelace", "李四", "José Ñuñez", "O'Brien-Smith", "Ω μ 😀",
	}
	for _, input := range accepted {
		var v validate.Violations
		if !v.NoControlChars("name", input) {
			t.Errorf("%q was rejected but is a legitimate name", input)
		}
	}
}
