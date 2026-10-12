package auth

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/validate"
)

// PasswordResetToken is a stored, one-time password reset token.
type PasswordResetToken struct {
	ID        string
	UserID    string
	TokenHash []byte
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// Usable reports whether the token may still be redeemed.
func (t PasswordResetToken) Usable(now time.Time) bool {
	return t.UsedAt == nil && now.Before(t.ExpiresAt)
}

// NewPasswordResetToken describes a token to be created.
type NewPasswordResetToken struct {
	ID        string
	UserID    string
	TokenHash []byte
	ExpiresAt time.Time
}

// PasswordResetRepository stores password reset tokens.
type PasswordResetRepository interface {
	Create(ctx context.Context, t NewPasswordResetToken) error
	// GetByTokenHash takes a row lock — see queries/password_reset.sql's
	// GetPasswordResetTokenByHash for why: ConfirmPasswordReset reads then
	// writes back (marks it used), and two concurrent confirmations of the
	// same token must not both succeed.
	GetByTokenHash(ctx context.Context, hash []byte) (PasswordResetToken, error)
	MarkUsed(ctx context.Context, id string) (bool, error)
	// InvalidatePendingForUser removes any earlier unconsumed token before
	// a new one is issued, so requesting a reset twice cannot leave two
	// live links.
	InvalidatePendingForUser(ctx context.Context, userID string) (int64, error)
}

// NewPasswordResetTokenID mints an identifier for a password reset token.
func NewPasswordResetTokenID() string { return uuid.Must(uuid.NewV7()).String() }

// EventPasswordResetRequested is emitted when a member requests a password
// reset. It is the event package notify sends the reset email for.
const EventPasswordResetRequested = "user.password_reset_requested.v1"

// PasswordResetRequestedPayload is the body of EventPasswordResetRequested.
//
// Token is the AES-256-GCM encryption of the plaintext (TokenCipher.
// Encrypt), never the plaintext itself — see
// invite.InvitationCreatedPayload's doc comment for the identical reasoning:
// the outbox is a generic, admin-readable table, and a redeemable
// credential has no business sitting in it even encrypted-at-rest-only.
type PasswordResetRequestedPayload struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Token  string `json:"token"`
}

// TokenEncrypter encrypts the token this package puts on the wire. Declared
// as an interface, even though *TokenCipher is the only implementation
// anyone wires up, so tests can substitute a no-op instead of exercising
// real AES-GCM — matching invite.TokenEncrypter's reasoning.
type TokenEncrypter interface {
	Encrypt(plaintext string) (string, error)
}

// EventPublisher records a domain event — declared here rather than
// depending on messaging.Publisher directly, matching every other
// feature's identically-shaped interface (ADR-0002).
type EventPublisher interface {
	Publish(ctx context.Context, event messaging.Event) error
}

// AuditEntry is a minimal, pure-data description of a security-sensitive
// action, passed to AuditRecorder.
//
// It cannot reuse audit.Entry the way every other feature package does
// (see ticket.go's AuditRecorder, for instance): package audit itself
// depends on auth for auth.Authorize's permission checks, so auth
// importing audit back would be an import cycle. cmd/api adapts between
// the two at the composition root — see its AuditEntry-to-audit.Entry
// conversion next to authService.SetAuditRecorder.
type AuditEntry struct {
	ActorID    *string
	Action     string
	TargetType string
	TargetID   string
	After      any
}

// AuditRecorder records a security-sensitive action for the audit trail.
type AuditRecorder interface {
	Record(ctx context.Context, e AuditEntry) error
}

type nopAuditRecorder struct{}

func (nopAuditRecorder) Record(context.Context, AuditEntry) error { return nil }

// SetAuditRecorder attaches the audit trail. Passing nil restores the
// no-op.
func (s *Service) SetAuditRecorder(a AuditRecorder) {
	if a == nil {
		a = nopAuditRecorder{}
	}
	s.audit = a
}

// RequestPasswordReset issues a one-time reset link if, and only if,
// identifier matches an active account — but the response is identical
// either way, and so is the error (there is none): the caller must not be
// able to tell "an email is on its way" apart from "there is no such
// account" from the response alone. The account-enumeration guarantee this
// makes is about the response, not about timing — see the type's own
// package doc for that scope decision.
//
// A disabled account is treated the same as no account: no token is
// issued, and nothing about that is visible to the caller.
func (s *Service) RequestPasswordReset(ctx context.Context, identifier string) error {
	var v validate.Violations
	v.Required("identifier", identifier)
	if v.Any() {
		return v.Err("INVALID_ARGUMENT", "invalid password reset request")
	}

	account, err := s.accounts.FindByIdentifier(ctx, validate.NormalizeIdentifier(identifier))
	switch {
	case err == nil:
	case apperr.Is(err, apperr.KindNotFound):
		return nil
	default:
		return err
	}
	if !account.Active {
		return nil
	}

	token, digest, err := NewRefreshToken()
	if err != nil {
		return apperr.Internal(err)
	}
	encryptedToken, err := s.resetCipher.Encrypt(token)
	if err != nil {
		return apperr.Internal(err)
	}

	now := s.now()
	return s.tx.WithTx(ctx, func(ctx context.Context) error {
		if _, err := s.resets.InvalidatePendingForUser(ctx, account.UserID); err != nil {
			return err
		}
		if err := s.resets.Create(ctx, NewPasswordResetToken{
			ID: NewPasswordResetTokenID(), UserID: account.UserID, TokenHash: digest,
			ExpiresAt: now.Add(s.cfg.PasswordResetTTL),
		}); err != nil {
			return err
		}

		event, err := messaging.NewEvent(EventPasswordResetRequested, PasswordResetRequestedPayload{
			UserID: account.UserID, Email: account.Email, Token: encryptedToken,
		})
		if err != nil {
			return apperr.Internal(err)
		}
		event.TraceParent = messaging.TraceParentFromContext(ctx)
		if err := s.events.Publish(ctx, event); err != nil {
			return apperr.Internal(err)
		}

		return s.audit.Record(ctx, AuditEntry{
			ActorID: &account.UserID, Action: "password_reset.requested",
			TargetType: "user", TargetID: account.UserID,
		})
	})
}

// errPasswordResetTokenInvalid is the single answer to every reset-token
// failure — unknown, expired, or already used — for the same reason
// errInvalidCredentials is: distinguishing them would tell an attacker
// which guess was closest.
func errPasswordResetTokenInvalid() error {
	return apperr.Invalid("PASSWORD_RESET_TOKEN_INVALID", "this reset link is invalid or has expired")
}

// ConfirmPasswordReset redeems a one-time token and sets a new password.
//
// Every session the account holds is revoked, with no exception: unlike a
// self-service session revoke, there is no "current session" to preserve —
// the caller is, by definition, not authenticated yet — and a password
// reset is exactly the moment every previously-issued refresh token should
// stop working. The access token(s) already handed out for this account
// keep working until their own natural expiry regardless (at most
// AUTH_ACCESS_TTL, capped at 1h — see ADR-0007); that limit is what this
// method cannot shorten, not a gap specific to reset.
func (s *Service) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	var v validate.Violations
	v.Required("token", token)
	if v.Any() {
		return v.Err("INVALID_ARGUMENT", "invalid password reset confirmation")
	}
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}

	newHash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return apperr.Internal(err)
	}

	var rejected bool
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		record, err := s.resets.GetByTokenHash(ctx, HashRefreshToken(token))
		switch {
		case err == nil:
		case apperr.Is(err, apperr.KindNotFound):
			rejected = true
			return nil
		default:
			return err
		}
		if !record.Usable(s.now()) {
			rejected = true
			return nil
		}

		// The row lock GetByTokenHash took above is what makes this safe:
		// a second, concurrent confirmation of the same token blocks until
		// this transaction commits, then re-reads the row and finds
		// UsedAt already set — so it fails the Usable check above before
		// ever reaching here, the same way Refresh's re-check of
		// session.RevokedAt after its own locked read does.
		if err := s.accounts.UpdatePasswordHash(ctx, record.UserID, newHash); err != nil {
			return err
		}
		if _, err := s.resets.MarkUsed(ctx, record.ID); err != nil {
			return err
		}
		if _, err := s.sessions.RevokeAllForUser(ctx, record.UserID); err != nil {
			return err
		}

		return s.audit.Record(ctx, AuditEntry{
			ActorID: &record.UserID, Action: "password_reset.confirmed",
			TargetType: "user", TargetID: record.UserID,
		})
	})
	if err != nil {
		return err
	}
	if rejected {
		return errPasswordResetTokenInvalid()
	}
	return nil
}
