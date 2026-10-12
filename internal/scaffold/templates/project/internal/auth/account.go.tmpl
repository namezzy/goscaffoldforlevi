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
	Email        string
	PasswordHash string
	Role         Role
	// Active is false for accounts that exist but may not authenticate. The
	// distinction is not exposed to the client — see Service.Login.
	Active bool
}

// AccountStore reads accounts and writes back re-hashed passwords. It is
// implemented in the user package, which owns the storage.
type AccountStore interface {
	// FindByIdentifier looks up an account for login by email or username —
	// whichever the already-normalised identifier matches.
	FindByIdentifier(ctx context.Context, identifier string) (Account, error)
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
	// ListRecentByUser backs the security center's session list — one row
	// per login family (see queries/auth.sql's
	// ListRecentRefreshSessionsByUser for why per-family, not per
	// refresh-token rotation), newest first. See Service.ListSessions.
	ListRecentByUser(ctx context.Context, userID string, limit int32) ([]Session, error)
	// RevokeOwned revokes one session (one family), scoped to userID so a
	// caller can never revoke a session that is not theirs by guessing its
	// id. The bool reports whether anything was actually revoked — false
	// covers both "no such session" and "not yours" identically, on
	// purpose (see Service.RevokeSession).
	RevokeOwned(ctx context.Context, family FamilyID, userID string) (bool, error)
	// RevokeAllForUserExceptFamily backs "revoke my other sessions".
	RevokeAllForUserExceptFamily(ctx context.Context, userID string, exceptFamily FamilyID) (int64, error)
	// RevokeAllForUser revokes every active session a user holds, across
	// every login family. Declared here (not only on the concrete
	// *authpg.SessionRepository, as it was before Phase 3) because
	// Service.ConfirmPasswordReset now needs it directly, alongside
	// user.SessionRevoker's existing use for member deactivation.
	RevokeAllForUser(ctx context.Context, userID string) (int64, error)
}

// TxManager runs a function inside a database transaction. The auth service
// depends on this two-method-free interface rather than on pgx so that the
// domain layer stays free of driver types (ADR-0002).
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Clock is injected so that expiry and rotation can be tested without sleeping.
type Clock func() time.Time
