package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AccessClaims is the payload of an access token.
//
// It carries no personally identifiable information. A JWT is only signed, not
// encrypted, so anything in it is readable by anyone who sees the token —
// including browser storage, proxy logs and crash reports. Role is included
// despite that, because it is not PII and its absence would cost a database
// lookup on every request.
type AccessClaims struct {
	Issuer    string
	Subject   string // user ID
	Audience  string
	IssuedAt  time.Time
	ExpiresAt time.Time
	// TokenID is the jti. It is mandatory: without a unique identifier there is
	// no way to revoke a single token or to attribute an audited action to one.
	TokenID string
	Role    Role
}

// TokenIssuer mints access tokens.
//
// Services depend on this interface rather than on a JWT library so that the
// token format is a detail of one package. Swapping HS256 for asymmetric keys,
// or JWT for opaque tokens, then touches the implementation and nothing else.
type TokenIssuer interface {
	Issue(claims AccessClaims) (string, error)
}

// TokenVerifier validates an access token and returns its claims.
type TokenVerifier interface {
	// Verify returns ErrInvalidToken for every failure mode. Callers must not
	// be able to tell an expired token from a forged one.
	Verify(token string) (AccessClaims, error)
}

// RefreshTokenLen is the size of the random part of a refresh token.
//
// 32 bytes is 256 bits of entropy. Because the value is uniformly random rather
// than user-chosen, it needs no key stretching: guessing it is infeasible by
// construction, so hashing it with Argon2 would only add latency to a request
// that happens on every session renewal.
const RefreshTokenLen = 32

// NewRefreshToken returns a fresh opaque token and its storage digest.
//
// Only the digest is persisted. A database leak then yields values that cannot
// be replayed, which is the same reasoning as for passwords even though the
// hash function differs.
func NewRefreshToken() (token string, digest []byte, err error) {
	raw := make([]byte, RefreshTokenLen)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("auth: generate refresh token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashRefreshToken(token), nil
}

// HashRefreshToken derives the storage digest of a refresh token.
func HashRefreshToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// TokenPair is what the login and refresh endpoints return.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	// ExpiresIn is the access token lifetime in seconds. Clients need it to
	// schedule renewal without parsing the token, which they should not do.
	ExpiresIn int
}

// SessionID identifies a refresh session row.
type SessionID uuid.UUID

// NewSessionID mints an identifier for a refresh session.
func NewSessionID() SessionID { return SessionID(uuid.Must(uuid.NewV7())) }

// UUID exposes the underlying value for the persistence layer.
func (id SessionID) UUID() uuid.UUID { return uuid.UUID(id) }

func (id SessionID) String() string { return uuid.UUID(id).String() }

// FamilyID groups every token descended from a single login. Rotation keeps the
// family constant, so revoking a family logs out one device without touching
// the user's other sessions.
type FamilyID uuid.UUID

// NewFamilyID mints an identifier for a new session family, which is created
// once per login and preserved across every rotation that follows.
func NewFamilyID() FamilyID { return FamilyID(uuid.Must(uuid.NewV7())) }

// UUID exposes the underlying value for the persistence layer.
func (id FamilyID) UUID() uuid.UUID { return uuid.UUID(id) }

func (id FamilyID) String() string { return uuid.UUID(id).String() }
