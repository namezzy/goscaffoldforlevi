// Package authjwt implements auth.TokenIssuer and auth.TokenVerifier on top of
// JWT.
//
// It is separate from the auth package so that the JWT library appears in
// exactly one import graph. Services depend on the interfaces; only the
// composition root knows this package exists.
package authjwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
)

// minSecretLen matches the HMAC-SHA256 block size. A shorter key does not make
// the MAC weaker in a way an attacker can exploit directly, but it is a
// reliable sign the secret is a password someone typed rather than random
// bytes, and those are guessable.
const minSecretLen = 32

// HMAC implements both auth.TokenIssuer and auth.TokenVerifier using HS256.
//
// HS256 is chosen over RS256 because a single service both issues and verifies
// these tokens; asymmetric keys only pay off when verification happens
// somewhere the signing key must not be. That trade-off is recorded in
// ADR-0007 and is expected to change if a second service starts verifying.
type HMAC struct {
	secret   []byte
	issuer   string
	audience string
	// leeway absorbs clock skew between the issuing and verifying host. Without
	// it, a token issued on a host a second ahead is rejected as not-yet-valid.
	leeway time.Duration
}

// New builds an HMAC signer. It rejects short secrets rather than accepting
// them, because a misconfigured secret produces tokens that look fine.
func New(secret, issuer, audience string) (*HMAC, error) {
	if len(secret) < minSecretLen {
		return nil, fmt.Errorf("authjwt: secret must be at least %d bytes", minSecretLen)
	}
	if issuer == "" || audience == "" {
		return nil, errors.New("authjwt: issuer and audience are required")
	}
	return &HMAC{
		secret:   []byte(secret),
		issuer:   issuer,
		audience: audience,
		leeway:   30 * time.Second,
	}, nil
}

type claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// Issue mints a signed access token.
func (h *HMAC) Issue(c auth.AccessClaims) (string, error) {
	if c.TokenID == "" {
		// A token without a jti cannot be individually identified in an audit
		// trail, so an empty one is a programming error rather than a default.
		return "", errors.New("authjwt: token id is required")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Role: string(c.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    c.Issuer,
			Subject:   c.Subject,
			Audience:  jwt.ClaimStrings{c.Audience},
			IssuedAt:  jwt.NewNumericDate(c.IssuedAt),
			ExpiresAt: jwt.NewNumericDate(c.ExpiresAt),
			ID:        c.TokenID,
		},
	})

	signed, err := token.SignedString(h.secret)
	if err != nil {
		return "", fmt.Errorf("authjwt: sign: %w", err)
	}
	return signed, nil
}

// Verify checks the signature and the registered claims.
//
// Every failure returns auth.ErrInvalidToken with no detail. Distinguishing
// "expired" from "bad signature" would let an attacker probing forged tokens
// learn whether the structure was right, and would tell a client nothing it can
// act on beyond "authenticate again".
func (h *HMAC) Verify(raw string) (auth.AccessClaims, error) {
	var c claims

	// The parser is configured to accept exactly one algorithm. Leaving that
	// open is the classic JWT vulnerability: a token whose header says "none",
	// or an RS256 verifier fed an HS256 token signed with the public key, both
	// verify successfully against a permissive parser.
	_, err := jwt.ParseWithClaims(raw, &c, func(*jwt.Token) (any, error) {
		return h.secret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(h.issuer),
		jwt.WithAudience(h.audience),
		jwt.WithLeeway(h.leeway),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return auth.AccessClaims{}, auth.ErrInvalidToken()
	}

	// The library validates the claims it knows about; these two it does not.
	// A token with no subject would authenticate a caller with an empty user
	// ID, and one with no jti cannot be audited.
	if c.Subject == "" || c.ID == "" {
		return auth.AccessClaims{}, auth.ErrInvalidToken()
	}

	role := auth.Role(c.Role)
	if !role.Valid() {
		return auth.AccessClaims{}, auth.ErrInvalidToken()
	}

	out := auth.AccessClaims{
		Issuer:   c.Issuer,
		Subject:  c.Subject,
		Audience: h.audience,
		TokenID:  c.ID,
		Role:     role,
	}
	if c.IssuedAt != nil {
		out.IssuedAt = c.IssuedAt.Time
	}
	if c.ExpiresAt != nil {
		out.ExpiresAt = c.ExpiresAt.Time
	}
	return out, nil
}
