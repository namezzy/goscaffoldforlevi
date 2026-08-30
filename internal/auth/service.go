package auth

import (
	"context"

	"log/slog"
	"net/netip"
	"time"

	"github.com/example/go-enterprise/internal/apperr"
	"github.com/example/go-enterprise/internal/validate"
)

// PasswordHasher is the subset of Argon2Hasher the service needs. Depending on
// the interface keeps the service testable without paying 64 MiB per test case.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, encoded string) VerifyResult
	VerifyDummy()
}

// ServiceConfig holds the token lifetimes.
type ServiceConfig struct {
	Issuer     string
	Audience   string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

// Service performs authentication. It owns no HTTP or SQL knowledge: the
// request metadata it records (user agent, IP) arrives as plain values.
type Service struct {
	cfg      ServiceConfig
	accounts AccountStore
	sessions SessionRepository
	hasher   PasswordHasher
	issuer   TokenIssuer
	tx       TxManager
	now      Clock
	log      *slog.Logger
}

// NewService wires the service. Every collaborator is an interface, so the
// composition root decides what backs each one.
func NewService(
	cfg ServiceConfig,
	accounts AccountStore,
	sessions SessionRepository,
	hasher PasswordHasher,
	issuer TokenIssuer,
	tx TxManager,
	log *slog.Logger,
) *Service {
	return &Service{
		cfg:      cfg,
		accounts: accounts,
		sessions: sessions,
		hasher:   hasher,
		issuer:   issuer,
		tx:       tx,
		now:      time.Now,
		log:      log,
	}
}

// SetClock overrides the time source. Tests use it to advance past expiry
// without sleeping.
func (s *Service) SetClock(c Clock) { s.now = c }

// RequestMeta is the non-secret context of an authentication request, recorded
// on the session so a user can later recognise their own devices.
type RequestMeta struct {
	UserAgent string
	IP        netip.Addr
}

// errInvalidCredentials is the single answer to every login failure.
//
// Unknown email, wrong password and disabled account all produce this exact
// value. Any difference — in code, message, or status — turns the endpoint into
// an account enumeration oracle, and "this account is disabled" is precisely
// the confirmation an attacker wants.
func errInvalidCredentials() error {
	return apperr.Unauthorized("INVALID_CREDENTIALS", "invalid email or password")
}

// Login exchanges an email and password for a token pair.
//
// The three failure paths are deliberately kept indistinguishable not only in
// their response but in their cost: when no account matches, the service still
// performs a full Argon2id verification against a dummy hash, and a disabled
// account is rejected only after its real password has been checked. Skipping
// either would make the timing of a failure reveal which case it was.
func (s *Service) Login(ctx context.Context, email, password string, meta RequestMeta) (TokenPair, error) {
	var v validate.Violations
	v.Required("email", email)
	v.Required("password", password)
	if v.Any() {
		return TokenPair{}, v.Err("INVALID_ARGUMENT", "invalid login request")
	}

	account, err := s.accounts.FindByEmail(ctx, validate.NormalizeEmail(email))
	switch {
	case err == nil:
	case apperr.Is(err, apperr.KindNotFound):
		s.hasher.VerifyDummy()
		return TokenPair{}, errInvalidCredentials()
	default:
		return TokenPair{}, err
	}

	result := s.hasher.Verify(password, account.PasswordHash)
	if !result.OK {
		return TokenPair{}, errInvalidCredentials()
	}
	if !account.Active {
		return TokenPair{}, errInvalidCredentials()
	}

	// Login is the only moment the plaintext password exists, so it is the only
	// chance to migrate a user onto stronger parameters. A failure here must
	// not fail the login: the user's credentials were correct, and refusing
	// them because of a maintenance task would be worse than staying on the old
	// cost for one more session.
	if result.NeedsRehash {
		if newHash, hashErr := s.hasher.Hash(password); hashErr == nil {
			if updErr := s.accounts.UpdatePasswordHash(ctx, account.UserID, newHash); updErr != nil {
				s.log.WarnContext(ctx, "password rehash failed", slog.String("user_id", account.UserID))
			}
		}
	}

	return s.issuePair(ctx, account.UserID, account.Role, NewFamilyID(), nil, meta)
}

// Refresh rotates a refresh token.
//
// Rotation is what makes a stolen refresh token detectable: the legitimate
// client discards its token after use, so a second presentation of an already
// rotated token can only come from a copy. When that happens the whole family
// is revoked, which logs out both the attacker and the victim — deliberately,
// because the alternative is leaving an attacker with a valid session.
func (s *Service) Refresh(ctx context.Context, refreshToken string, meta RequestMeta) (TokenPair, error) {
	if refreshToken == "" {
		return TokenPair{}, ErrInvalidToken()
	}

	// rejected separates "this token is not usable" from "this operation
	// failed". The distinction matters because returning an error from inside
	// WithTx rolls the transaction back — which would discard the very
	// revocations performed below. The rejection is therefore carried out of
	// the closure as a value, and turned into an error only after the
	// transaction has committed.
	var (
		pair     TokenPair
		rejected bool
	)
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		session, err := s.sessions.GetByTokenHash(ctx, HashRefreshToken(refreshToken))
		switch {
		case err == nil:
		case apperr.Is(err, apperr.KindNotFound):
			rejected = true
			return nil
		default:
			return err
		}

		now := s.now()

		if session.RevokedAt != nil {
			// A revoked token was presented. Either it was stolen and replayed,
			// or the user logged out and something retried; both warrant
			// killing the family, and the cost of a false positive is one
			// re-login.
			revoked, revokeErr := s.sessions.RevokeFamily(ctx, session.FamilyID)
			if revokeErr != nil {
				return revokeErr
			}
			s.log.WarnContext(ctx, "refresh token reuse detected, family revoked",
				slog.String("user_id", session.UserID),
				slog.String("family_id", session.FamilyID.String()),
				slog.Int64("sessions_revoked", revoked),
			)
			rejected = true
			return nil
		}

		if !now.Before(session.ExpiresAt) {
			rejected = true
			return nil
		}

		// The account is re-read on every rotation. The role or the account's
		// status may have changed since login, and a refresh token that kept
		// working for a disabled user would make deactivation take up to the
		// refresh lifetime to have any effect.
		account, err := s.accounts.FindByID(ctx, session.UserID)
		switch {
		case err == nil:
		case apperr.Is(err, apperr.KindNotFound):
			rejected = true
			return nil
		default:
			return err
		}
		if !account.Active {
			if _, revokeErr := s.sessions.RevokeFamily(ctx, session.FamilyID); revokeErr != nil {
				return revokeErr
			}
			rejected = true
			return nil
		}

		// Revoke before issuing. If the process dies between the two, the user
		// has to log in again; doing it the other way round could leave two
		// live tokens in the same family, which is the state reuse detection
		// assumes is impossible.
		if err := s.sessions.Revoke(ctx, session.ID); err != nil {
			return err
		}

		parent := session.ID
		pair, err = s.issuePair(ctx, account.UserID, account.Role, session.FamilyID, &parent, meta)
		return err
	})
	if err != nil {
		return TokenPair{}, err
	}
	if rejected {
		return TokenPair{}, ErrInvalidToken()
	}
	return pair, nil
}

// Logout revokes the entire family the presented token belongs to.
//
// Revoking only the presented token would leave every token already rotated
// from it usable, and the user's expectation of "log out" is that this device
// stops working — which is exactly what a family is.
//
// An unknown token succeeds silently. Logout is not an authentication
// endpoint, so reporting "no such session" would let an unauthenticated caller
// probe which token values exist, and a client that cannot log out because its
// token already expired has no useful recovery action.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	return s.tx.WithTx(ctx, func(ctx context.Context) error {
		session, err := s.sessions.GetByTokenHash(ctx, HashRefreshToken(refreshToken))
		switch {
		case err == nil:
		case apperr.Is(err, apperr.KindNotFound):
			return nil
		default:
			return err
		}
		_, err = s.sessions.RevokeFamily(ctx, session.FamilyID)
		return err
	})
}

// issuePair mints an access token and a refresh token and records the new
// session. Both tokens are produced together so that a caller can never end up
// holding one without the other.
func (s *Service) issuePair(
	ctx context.Context,
	userID string,
	role Role,
	family FamilyID,
	parent *SessionID,
	meta RequestMeta,
) (TokenPair, error) {
	now := s.now()

	refreshToken, digest, err := NewRefreshToken()
	if err != nil {
		return TokenPair{}, apperr.Internal(err)
	}

	newSession := NewSession{
		ID:        NewSessionID(),
		UserID:    userID,
		TokenHash: digest,
		FamilyID:  family,
		ParentID:  parent,
		ExpiresAt: now.Add(s.cfg.RefreshTTL),
		UserAgent: truncatedUserAgent(meta.UserAgent),
	}
	if meta.IP.IsValid() {
		ip := meta.IP
		newSession.IP = &ip
	}

	if _, err := s.sessions.Create(ctx, newSession); err != nil {
		return TokenPair{}, err
	}

	access, err := s.issuer.Issue(AccessClaims{
		Issuer:    s.cfg.Issuer,
		Subject:   userID,
		Audience:  s.cfg.Audience,
		IssuedAt:  now,
		ExpiresAt: now.Add(s.cfg.AccessTTL),
		TokenID:   NewSessionID().String(),
		Role:      role,
	})
	if err != nil {
		return TokenPair{}, apperr.Internal(err)
	}

	return TokenPair{
		AccessToken:  access,
		RefreshToken: refreshToken,
		ExpiresIn:    int(s.cfg.AccessTTL.Seconds()),
	}, nil
}

// maxUserAgentLen bounds what is copied into the database. The header is
// attacker-controlled and unbounded in practice.
const maxUserAgentLen = 256

func truncatedUserAgent(ua string) *string {
	if ua == "" {
		return nil
	}
	if len(ua) > maxUserAgentLen {
		ua = ua[:maxUserAgentLen]
	}
	return &ua
}
