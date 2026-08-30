package auth_test

import (
	"strings"
	"testing"

	"github.com/example/go-enterprise/internal/apperr"
	"github.com/example/go-enterprise/internal/auth"
)

// testParams keep the tests fast. They are below the production floor on
// purpose: what is being tested here is the encoding and the comparison logic,
// not the cost, and paying 64 MiB per case would make the suite unusable.
var testParams = auth.Argon2Params{Time: 1, Memory: 8 * 1024, Threads: 1, SaltLen: 16, KeyLen: 32}

func newTestHasher(t *testing.T) *auth.Argon2Hasher {
	t.Helper()
	h, err := auth.NewArgon2Hasher(testParams)
	if err != nil {
		t.Fatalf("NewArgon2Hasher() = %v", err)
	}
	return h
}

func TestHashProducesPHCEncoding(t *testing.T) {
	t.Parallel()

	h := newTestHasher(t)
	encoded, err := h.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash() = %v", err)
	}

	// The stored value has to be self-describing, or the parameters could never
	// be changed without invalidating every existing password.
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=8192,t=1,p=1$") {
		t.Errorf("Hash() = %q, want a PHC string carrying the parameters", encoded)
	}
	if n := strings.Count(encoded, "$"); n != 5 {
		t.Errorf("Hash() has %d separators, want 5", n)
	}
}

func TestHashIsSaltedPerCall(t *testing.T) {
	t.Parallel()

	h := newTestHasher(t)
	const password = "correct horse battery staple"

	first, err := h.Hash(password)
	if err != nil {
		t.Fatalf("Hash() = %v", err)
	}
	second, err := h.Hash(password)
	if err != nil {
		t.Fatalf("Hash() = %v", err)
	}

	// Identical passwords must not produce identical hashes, or a leaked table
	// immediately reveals which accounts share a password.
	if first == second {
		t.Error("Hash() produced the same value twice; the salt is not random")
	}
	if !h.Verify(password, first).OK || !h.Verify(password, second).OK {
		t.Error("Verify() rejected a hash this hasher produced")
	}
}

func TestVerifyRejectsWrongPassword(t *testing.T) {
	t.Parallel()

	h := newTestHasher(t)
	encoded, err := h.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash() = %v", err)
	}

	if h.Verify("correct horse battery stapl", encoded).OK {
		t.Error("Verify() accepted a wrong password")
	}
}

func TestVerifyRejectsMalformedHashesWithoutPanicking(t *testing.T) {
	t.Parallel()

	h := newTestHasher(t)

	// A corrupted row must fail closed. Returning an error that a caller might
	// mishandle, or panicking, would turn bad data into an outage or a bypass.
	cases := map[string]string{
		"empty":              "",
		"not phc":            "plaintext",
		"wrong algorithm":    "$argon2i$v=19$m=8192,t=1,p=1$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNoaGFzaGhhc2hoYXNoaGE",
		"unsupported verion": "$argon2id$v=16$m=8192,t=1,p=1$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNoaGFzaGhhc2hoYXNoaGE",
		"bad base64":         "$argon2id$v=19$m=8192,t=1,p=1$!!!$!!!",
		"missing fields":     "$argon2id$v=19$m=8192,t=1,p=1",
		"empty salt":         "$argon2id$v=19$m=8192,t=1,p=1$$aGFzaA",
	}

	for name, encoded := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if h.Verify("any password at all", encoded).OK {
				t.Errorf("Verify(%q) reported success", encoded)
			}
		})
	}
}

func TestVerifyReportsRehashWhenStoredCostIsLower(t *testing.T) {
	t.Parallel()

	weak, err := auth.NewArgon2Hasher(auth.Argon2Params{
		Time: 1, Memory: 8 * 1024, Threads: 1, SaltLen: 16, KeyLen: 32,
	})
	if err != nil {
		t.Fatalf("NewArgon2Hasher() = %v", err)
	}
	strong, err := auth.NewArgon2Hasher(auth.Argon2Params{
		Time: 2, Memory: 16 * 1024, Threads: 1, SaltLen: 16, KeyLen: 32,
	})
	if err != nil {
		t.Fatalf("NewArgon2Hasher() = %v", err)
	}

	const password = "correct horse battery staple"
	old, err := weak.Hash(password)
	if err != nil {
		t.Fatalf("Hash() = %v", err)
	}

	// Login is the only moment the plaintext exists, so the upgrade has to be
	// detectable exactly here.
	res := strong.Verify(password, old)
	if !res.OK {
		t.Fatal("Verify() rejected a correct password hashed with older parameters")
	}
	if !res.NeedsRehash {
		t.Error("Verify().NeedsRehash = false, want true for a weaker stored hash")
	}

	upgraded, err := strong.Hash(password)
	if err != nil {
		t.Fatalf("Hash() = %v", err)
	}
	if strong.Verify(password, upgraded).NeedsRehash {
		t.Error("Verify().NeedsRehash = true for a hash at the current cost")
	}
}

func TestValidatePasswordEnforcesLengthPolicy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"too short", strings.Repeat("a", auth.MinPasswordLen-1), true},
		{"at minimum", strings.Repeat("a", auth.MinPasswordLen), false},
		{"at maximum", strings.Repeat("a", auth.MaxPasswordLen), false},
		// The upper bound is a denial-of-service control: Argon2 hashes the
		// whole input, so an unbounded password is unbounded work.
		{"too long", strings.Repeat("a", auth.MaxPasswordLen+1), true},
		// No character-class rule exists, so a long lowercase passphrase is
		// acceptable by design.
		{"passphrase with no symbols", "correct horse battery staple", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := auth.ValidatePassword(tc.password)
			if tc.wantErr {
				if err == nil {
					t.Fatal("ValidatePassword() = nil, want an error")
				}
				if !apperr.Is(err, apperr.KindInvalidArgument) {
					t.Errorf("ValidatePassword() kind = %v, want invalid_argument", apperr.KindOf(err))
				}
				return
			}
			if err != nil {
				t.Errorf("ValidatePassword() = %v, want nil", err)
			}
		})
	}
}

func TestNewArgon2HasherRejectsWeakParameters(t *testing.T) {
	t.Parallel()

	cases := map[string]auth.Argon2Params{
		"no iterations": {Time: 0, Memory: 8 * 1024, Threads: 1, SaltLen: 16, KeyLen: 32},
		"tiny memory":   {Time: 1, Memory: 64, Threads: 1, SaltLen: 16, KeyLen: 32},
		"short salt":    {Time: 1, Memory: 8 * 1024, Threads: 1, SaltLen: 8, KeyLen: 32},
		"short key":     {Time: 1, Memory: 8 * 1024, Threads: 1, SaltLen: 16, KeyLen: 16},
	}

	for name, params := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := auth.NewArgon2Hasher(params); err == nil {
				t.Error("NewArgon2Hasher() = nil error, want a rejection")
			}
		})
	}
}
