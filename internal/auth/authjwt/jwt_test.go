package authjwt_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/auth/authjwt"
)

const (
	testSecret   = "a-genuinely-random-32-byte-secret!!"
	testIssuer   = "test-issuer"
	testAudience = "test-audience"
)

func newSigner(t *testing.T) *authjwt.HMAC {
	t.Helper()
	h, err := authjwt.New(testSecret, testIssuer, testAudience)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	return h
}

func validClaims() auth.AccessClaims {
	now := time.Now()
	return auth.AccessClaims{
		Issuer:    testIssuer,
		Subject:   "01930f8e-0000-7000-8000-000000000001",
		Audience:  testAudience,
		IssuedAt:  now,
		ExpiresAt: now.Add(15 * time.Minute),
		TokenID:   "01930f8e-0000-7000-8000-0000000000ff",
		Role:      auth.RoleAdmin,
	}
}

func TestIssueAndVerifyRoundTrip(t *testing.T) {
	t.Parallel()

	h := newSigner(t)
	want := validClaims()

	token, err := h.Issue(want)
	if err != nil {
		t.Fatalf("Issue() = %v", err)
	}

	got, err := h.Verify(token)
	if err != nil {
		t.Fatalf("Verify() = %v", err)
	}
	if got.Subject != want.Subject || got.Role != want.Role || got.TokenID != want.TokenID {
		t.Errorf("Verify() = %+v, want subject/role/jti to survive the round trip", got)
	}
}

func TestIssueRequiresATokenID(t *testing.T) {
	t.Parallel()

	claims := validClaims()
	claims.TokenID = ""

	if _, err := newSigner(t).Issue(claims); err == nil {
		t.Error("Issue() accepted claims with no jti")
	}
}

func TestTokenCarriesNoPersonalData(t *testing.T) {
	t.Parallel()

	token, err := newSigner(t).Issue(validClaims())
	if err != nil {
		t.Fatalf("Issue() = %v", err)
	}

	// A JWT is signed, not encrypted. Anything in the payload is readable by
	// anyone who obtains the token, so this test decodes it the way an attacker
	// would and asserts nothing sensitive is there.
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d segments, want 3", len(parts))
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	for _, forbidden := range []string{"email", "name", "password", "password_hash"} {
		if _, ok := payload[forbidden]; ok {
			t.Errorf("token payload contains %q", forbidden)
		}
	}
	for _, required := range []string{"iss", "sub", "aud", "iat", "exp", "jti", "role"} {
		if _, ok := payload[required]; !ok {
			t.Errorf("token payload is missing %q", required)
		}
	}
}

func TestVerifyRejectsTamperedAndForeignTokens(t *testing.T) {
	t.Parallel()

	h := newSigner(t)
	good, err := h.Issue(validClaims())
	if err != nil {
		t.Fatalf("Issue() = %v", err)
	}

	other, err := authjwt.New("a-different-32-byte-secret-value!!!", testIssuer, testAudience)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	foreign, err := other.Issue(validClaims())
	if err != nil {
		t.Fatalf("Issue() = %v", err)
	}

	cases := map[string]string{
		"empty":                   "",
		"not a jwt":               "definitely-not-a-token",
		"tampered payload":        mutateSegment(good, 1),
		"signed with another key": foreign,
	}

	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := h.Verify(token); err == nil {
				t.Error("Verify() accepted an invalid token")
			}
		})
	}
}

// TestVerifyRejectsAlgNone covers the classic JWT attack: a token whose header
// claims the algorithm is "none" and which therefore carries no signature. A
// parser that honours the header instead of the verifier's configuration
// accepts it, which is a complete authentication bypass.
func TestVerifyRejectsAlgNone(t *testing.T) {
	t.Parallel()

	claims := validClaims()
	unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
		Issuer:    claims.Issuer,
		Subject:   claims.Subject,
		Audience:  jwt.ClaimStrings{claims.Audience},
		IssuedAt:  jwt.NewNumericDate(claims.IssuedAt),
		ExpiresAt: jwt.NewNumericDate(claims.ExpiresAt),
		ID:        claims.TokenID,
	})
	token, err := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("build alg=none token: %v", err)
	}

	if _, err := newSigner(t).Verify(token); err == nil {
		t.Fatal("Verify() accepted an alg=none token")
	}
}

func TestVerifyEnforcesExpiryIssuerAndAudience(t *testing.T) {
	t.Parallel()

	h := newSigner(t)

	cases := []struct {
		name   string
		mutate func(*auth.AccessClaims)
	}{
		{"expired", func(c *auth.AccessClaims) {
			c.IssuedAt = time.Now().Add(-2 * time.Hour)
			c.ExpiresAt = time.Now().Add(-time.Hour)
		}},
		// An issuer or audience mismatch means the token was minted for a
		// different system. Accepting it would let a token from a sibling
		// service authenticate here.
		{"wrong issuer", func(c *auth.AccessClaims) { c.Issuer = "somebody-else" }},
		{"wrong audience", func(c *auth.AccessClaims) { c.Audience = "another-api" }},
		{"unknown role", func(c *auth.AccessClaims) { c.Role = "superadmin" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			claims := validClaims()
			tc.mutate(&claims)
			token, err := h.Issue(claims)
			if err != nil {
				t.Fatalf("Issue() = %v", err)
			}
			if _, err := h.Verify(token); err == nil {
				t.Error("Verify() accepted the token")
			}
		})
	}
}

func TestVerifyFailuresAreIndistinguishable(t *testing.T) {
	t.Parallel()

	h := newSigner(t)

	expired := validClaims()
	expired.ExpiresAt = time.Now().Add(-time.Hour)
	expiredToken, err := h.Issue(expired)
	if err != nil {
		t.Fatalf("Issue() = %v", err)
	}

	_, expiredErr := h.Verify(expiredToken)
	_, garbageErr := h.Verify("not.a.token")

	// Telling an expired token apart from a forged one turns the endpoint into
	// an oracle for whoever is probing it.
	if expiredErr == nil || garbageErr == nil {
		t.Fatal("Verify() accepted an invalid token")
	}
	if expiredErr.Error() != garbageErr.Error() {
		t.Errorf("failure messages differ: %q vs %q", expiredErr, garbageErr)
	}
}

func TestNewRejectsShortSecrets(t *testing.T) {
	t.Parallel()

	if _, err := authjwt.New("too-short", testIssuer, testAudience); err == nil {
		t.Error("New() accepted a short secret")
	}
	if _, err := authjwt.New(testSecret, "", testAudience); err == nil {
		t.Error("New() accepted an empty issuer")
	}
}

// mutateSegment flips a byte in one segment of a JWT so that the signature no
// longer matches.
func mutateSegment(token string, index int) string {
	parts := strings.Split(token, ".")
	segment := []byte(parts[index])
	if segment[0] == 'a' {
		segment[0] = 'b'
	} else {
		segment[0] = 'a'
	}
	parts[index] = string(segment)
	return strings.Join(parts, ".")
}
