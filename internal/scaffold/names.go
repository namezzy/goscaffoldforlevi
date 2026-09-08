// Package scaffold generates projects, modules and migrations from the
// templates embedded in this package.
//
// The templates are not maintained by hand. They are a copy of this
// repository's own files, produced by Sync and checked by a test that fails
// when the copy has fallen behind — the scaffold's output is this project,
// which is the only arrangement that keeps the two from drifting apart.
package scaffold

import (
	"fmt"
	"strings"
	"unicode"
)

// Name is one identifier expressed in every form the templates need.
//
// It is computed once and passed around, rather than converted at each use
// site, so that "order"/"Order"/"orders" can never disagree about which entity
// they refer to.
type Name struct {
	// Snake is the canonical, lower-case singular: the package name and the
	// name the user typed. "order_item".
	Snake string

	// Plural is the snake-case plural, used for table names and URL paths.
	// "order_items".
	Plural string

	// Pascal is the exported Go type name. "OrderItem".
	Pascal string

	// Camel is the unexported variable name. "orderItem".
	Camel string

	// Screaming is the error-code form. "ORDER_ITEM".
	Screaming string
}

// ParseName validates an identifier and derives every form from it.
//
// The accepted input is deliberately narrow: lower-case snake_case. A
// generator that accepts "OrderItem", "order-item" and "orderItem" alike has to
// guess which one the user meant, and it guesses wrong on exactly the inputs
// where it matters — acronyms, and words that are already plural.
func ParseName(s string) (Name, error) {
	if s == "" {
		return Name{}, fmt.Errorf("the name is empty")
	}
	for i, r := range s {
		switch {
		case unicode.IsLower(r):
		case r >= '0' && r <= '9' && i > 0:
		case r == '_' && i > 0 && i < len(s)-1:
		default:
			return Name{}, fmt.Errorf("%q is not a valid name: use lower-case snake_case, for example order_item", s)
		}
	}
	if strings.Contains(s, "__") {
		return Name{}, fmt.Errorf("%q is not a valid name: no repeated underscores", s)
	}

	words := strings.Split(s, "_")

	var pascal, camel strings.Builder
	for i, w := range words {
		if i == 0 {
			camel.WriteString(w)
		} else {
			camel.WriteString(title(w))
		}
		pascal.WriteString(title(w))
	}

	// Only the final word is pluralised: "order_item" becomes "order_items",
	// never "orders_items".
	plural := append(append([]string{}, words[:len(words)-1]...), pluralise(words[len(words)-1]))

	return Name{
		Snake:     s,
		Plural:    strings.Join(plural, "_"),
		Pascal:    pascal.String(),
		Camel:     camel.String(),
		Screaming: strings.ToUpper(s),
	}, nil
}

func title(w string) string {
	if w == "" {
		return w
	}
	r := []rune(w)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// pluralise implements the three English rules that cover ordinary domain
// nouns, and nothing else.
//
// An inflection library would handle "person" and "datum" too, at the cost of a
// dependency and a word list. The failure mode here is visible and trivially
// fixed — the generated file says "persons", and the developer renames it —
// whereas the library's failure mode is a surprising plural nobody predicted.
// If a project needs irregular plurals often, that is the signal to revisit.
func pluralise(w string) string {
	switch {
	case w == "":
		return w
	case strings.HasSuffix(w, "s"), strings.HasSuffix(w, "x"), strings.HasSuffix(w, "z"),
		strings.HasSuffix(w, "ch"), strings.HasSuffix(w, "sh"):
		return w + "es"
	case strings.HasSuffix(w, "y") && len(w) > 1 && !isVowel(rune(w[len(w)-2])):
		return w[:len(w)-1] + "ies"
	default:
		return w + "s"
	}
}

func isVowel(r rune) bool { return strings.ContainsRune("aeiou", r) }
