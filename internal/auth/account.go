package auth

import (
	"context"
	"net/netip"
	"time"
)

// Account is the slice of a user that authentication needs.
//
// Defining it here instead of importing the user entity keeps the dependency
// one-way (user -> auth) and makes the boundary explicit: if authentication
// ever needs another field, that has to be a deliberate change to this type
// rather than something that silently arrives with the user record.
type Account struct {
	UserID       string
	PasswordHash string
	Role         Role
	// Active is false for accounts that exist but may not authenticate. The
	// distinction is not exposed to the client — see Service.Login.
	Active bool
}

// AccountStore reads accounts and writes back re-hashed passwords. It is
// implemented in the user package, which owns the storage.
type AccountStore interface {
	FindByEmail(ctx context.Context, email string) (Account, error)
	FindByID(ctx context.Context, userID string) (Account, error)
	UpdatePasswordHash(ctx context.Context, userID, hash string) error
}

// Session is a stored refresh session.
type Session struct {
	ID        SessionID
	UserID    string
	TokenHash []byte
	FamilyID  FamilyID
	ParentID  *SessionID
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
	UserAgent *string
	IP        *netip.Addr
}

// Active reports whether the session may still be exchanged.
func (s Session) Active(now time.Time) bool {
	return s.RevokedAt == nil && now.Before(s.ExpiresAt)
}

// NewSession describes a session to be created.
type NewSession struct {
	ID        SessionID
	UserID    string
	TokenHash []byte
	FamilyID  FamilyID
	ParentID  *SessionID
	ExpiresAt time.Time
	UserAgent *string
	IP        *netip.Addr
}

// SessionRepository stores refresh sessions.
//
// Every method takes a context because rotation runs inside a transaction that
// is carried on the context; the repository must join that transaction rather
// than open its own connection, or the reuse check and the revocation would not
// be atomic.
type SessionRepository interface {
	Create(ctx context.Context, s NewSession) (Session, error)
	// GetByTokenHash takes a row lock. Callers always follow it with a write,
	// and two concurrent refreshes of the same token must not both win.
	GetByTokenHash(ctx context.Context, hash []byte) (Session, error)
	Revoke(ctx context.Context, id SessionID) error
	RevokeFamily(ctx context.Context, id FamilyID) (int64, error)
}

// TxManager runs a function inside a database transaction. The auth service
// depends on this two-method-free interface rather than on pgx so that the
// domain layer stays free of driver types (ADR-0002).
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Clock is injected so that expiry and rotation can be tested without sleeping.
type Clock func() time.Time
