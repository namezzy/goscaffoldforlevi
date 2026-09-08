// Package validate provides syntactic validation primitives.
//
// It deliberately contains no business rules: "password must be at least 12
// characters" belongs to the auth package, while "this string looks like an
// email address" belongs here.
package validate

import (
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
)

// Violation is a single field-level failure.
type Violation = apperr.Detail

// Violations accumulates field errors so a request can be reported in one
// response instead of forcing the client to fix one field per round trip.
type Violations struct {
	items []Violation
}

// Add records a violation for the given field.
func (v *Violations) Add(field, reason string) {
	v.items = append(v.items, Violation{Field: field, Reason: reason})
}

// Any reports whether at least one violation was recorded.
func (v *Violations) Any() bool { return len(v.items) > 0 }

// Items returns the recorded violations.
func (v *Violations) Items() []Violation { return v.items }

// Err returns nil when there are no violations, otherwise an
// apperr.KindInvalidArgument error carrying every violation as a detail.
func (v *Violations) Err(code, message string) error {
	if !v.Any() {
		return nil
	}
	return apperr.Invalid(code, message, v.items...)
}

// Required records a violation when s is empty after trimming.
func (v *Violations) Required(field, s string) bool {
	if strings.TrimSpace(s) == "" {
		v.Add(field, "required")
		return false
	}
	return true
}

// Email records a violation when s is not a syntactically valid address.
//
// This is intentionally permissive: the only reliable proof that an address
// exists is delivering mail to it. Over-strict regexes reject valid addresses.
func (v *Violations) Email(field, s string) bool {
	if !v.Required(field, s) {
		return false
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		v.Add(field, "invalid_email")
		return false
	}
	if !strings.Contains(strings.SplitN(s, "@", 2)[1], ".") {
		v.Add(field, "invalid_email")
		return false
	}
	return true
}

// RuneLen records a violation when s has fewer than min or more than max runes.
// Length is counted in runes, not bytes, so multi-byte names are not penalised.
func (v *Violations) RuneLen(field, s string, minLen, maxLen int) bool {
	n := utf8.RuneCountInString(s)
	switch {
	case n < minLen:
		v.Add(field, "too_short")
		return false
	case n > maxLen:
		v.Add(field, "too_long")
		return false
	}
	return true
}

// NoControlChars records a violation when s contains a character that has no
// place in human-entered text.
//
// The immediate reason is that PostgreSQL cannot store U+0000 in a text column
// at all: it answers SQLSTATE 22021, which reaches the client as an internal
// error for input the client supplied. Rejecting it here names the offending
// field instead.
//
// The rest of the set is about display rather than storage. Bidirectional
// overrides let "admin\u202egnp.exe" render as something else entirely, and
// invisible characters let two distinct names look identical in every user
// interface that shows them.
//
// Zero-width joiners are deliberately absent from the set: they are load-
// bearing in Persian and Indic names, and a rule that rejects real names to
// prevent a rendering trick has made the product worse.
func (v *Violations) NoControlChars(field, s string) bool {
	for _, r := range s {
		switch {
		case unicode.IsControl(r),
			r == '\u2028', r == '\u2029', // line and paragraph separators
			r >= '\u202a' && r <= '\u202e', // bidi embedding and override
			r >= '\u2066' && r <= '\u2069', // bidi isolates
			r == '\u200b',                  // zero-width space
			r == '\ufeff':                  // zero-width no-break space
			v.Add(field, "invalid_characters")
			return false
		}
	}
	return true
}

// OneOf records a violation when s is not among allowed.
func (v *Violations) OneOf(field, s string, allowed ...string) bool {
	for _, a := range allowed {
		if s == a {
			return true
		}
	}
	v.Add(field, "invalid_value")
	return false
}

// IntRange records a violation when n is outside [minVal, maxVal].
func (v *Violations) IntRange(field string, n, minVal, maxVal int) bool {
	if n < minVal || n > maxVal {
		v.Add(field, "out_of_range")
		return false
	}
	return true
}

// NormalizeEmail lowercases and trims an address so that comparisons and the
// database unique constraint agree on what "the same email" means.
//
// Only the domain is truly case-insensitive per RFC 5321, but every mail
// provider in practice treats the local part that way too, and treating
// "A@x.com" and "a@x.com" as different accounts is a security problem.
func NormalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
