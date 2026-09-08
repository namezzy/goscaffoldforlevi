package main

import (
	"os"
	"strings"
	"testing"
)

func TestGeneratedPasswordsAreDistinctAndLongEnough(t *testing.T) {
	seen := make(map[string]bool, 32)
	for range 32 {
		p, err := generatePassword()
		if err != nil {
			t.Fatalf("generate a password: %v", err)
		}
		if len(p) < 24 {
			t.Fatalf("generated password is only %d characters", len(p))
		}
		if seen[p] {
			t.Fatal("generatePassword returned a duplicate")
		}
		seen[p] = true
	}
}

func TestPasswordStdinDropsOnlyTheTrailingNewline(t *testing.T) {
	// A password piped in with `echo` arrives with a newline that is not part
	// of it, but internal whitespace is.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	original := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = original }()

	go func() {
		_, _ = w.WriteString("  pass word  \n")
		_ = w.Close()
	}()

	got, err := resolvePassword(true)
	if err != nil {
		t.Fatalf("resolvePassword: %v", err)
	}
	if got != "  pass word  " {
		t.Errorf("password = %q, want %q", got, "  pass word  ")
	}
	if strings.HasSuffix(got, "\n") {
		t.Error("the trailing newline was kept")
	}
}
