package auth

import (
	"context"
	"fmt"

	"log/slog"
	"net/netip"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/validate"
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
	// PasswordResetTTL is how long a self-service password reset token
	// stays usable after it is requested.
	PasswordResetTTL time.Duration
}

// Service performs authentication. It owns no HTTP or SQL knowledge: the
// request metadata it records (user agent, IP) arrives as plain values.
type Service struct {
	cfg         ServiceConfig
	accounts    AccountStore
	sessions    SessionRepository
	resets      PasswordResetRepository
	resetCipher TokenEncrypter
	hasher      PasswordHasher
	issuer      TokenIssuer
	events      EventPublisher
	tx          TxManager
	now         Clock
	log         *slog.Logger

	// observer and audit are never nil; see SetObserver and
	// SetAuditRecorder.
	observer Observer
	audit    AuditRecorder
}

// NewService wires the service. Every collaborator is an interface, so the
// composition root decides what backs each one. resets, resetCipher and
// events back the self-service password reset flow (Phase 3) — see
// RequestPasswordReset and ConfirmPasswordReset.
func NewService(
	cfg ServiceConfig,
	accounts AccountStore,
	sessions SessionRepository,
	resets PasswordResetRepository,
	resetCipher TokenEncrypter,
	hasher PasswordHasher,
	issuer TokenIssuer,
	events EventPublisher,
	tx TxManager,
	log *slog.Logger,
) *Service {
	return &Service{
		observer:    NopObserver(),
		audit:       nopAuditRecorder{},
		cfg:         cfg,
		accounts:    accounts,
		sessions:    sessions,
		resets:      resets,
		resetCipher: resetCipher,
		hasher:      hasher,
		issuer:      issuer,
		events:      events,
		tx:          tx,
		now:         time.Now,
		log:         log,
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
// Unknown identifier, wrong password and disabled account all produce this
// exact value. Any difference — in code, message, or status — turns the
// endpoint into an account enumeration oracle, and "this account is
// disabled" is precisely the confirmation an attacker wants.
func errInvalidCredentials() error {
	return apperr.Unauthorized("INVALID_CREDENTIALS", "invalid credentials")
}

// Login exchanges an identifier and password for a token pair. The
// identifier may be an email address or a username; both are accepted in the
// same field because a client asking a caller "email or username?" before it
// even knows whether the account exists is already leaking which one that
// account uses.
//
// The three failure paths are deliberately kept indistinguishable not only in
// their response but in their cost: when no account matches, the service still
// performs a full Argon2id verification against a dummy hash, and a disabled
// account is rejected only after its real password has been checked. Skipping
// either would make the timing of a failure reveal which case it was.
func (s *Service) Login(ctx context.Context, identifier, password string, meta RequestMeta) (TokenPair, error) {
	// The request is shaped before it is counted. A malformed body is a client
	// error, not a credential check, and counting it would let anyone raise
	// the authentication failure rate by posting empty forms — which turns a
	// security alert into a nuisance and then into something people mute.
	var v validate.Violations
	v.Required("identifier", identifier)
	v.Required("password", password)
	if v.Any() {
		return TokenPair{}, v.Err("INVALID_ARGUMENT", "invalid login request")
	}

	// Counted through one call on every terminating path, so that the failure
	// rate has attempts as its denominator. Successes as the denominator would
	// go to zero during the one incident that matters — every login failing —
	// and the alert would never fire.
	pair, err := s.login(ctx, identifier, password, meta)
	s.observer.LoginAttempt(err == nil)
	return pair, err
}

// SetObserver attaches instrumentation. Passing nil restores the no-op, so a
// caller that has not configured metrics cannot leave a nil interface on the
// login path.
func (s *Service) SetObserver(o Observer) {
	if o == nil {
		o = NopObserver()
	}
	s.observer = o
}

func (s *Service) login(ctx context.Context, identifier, password string, meta RequestMeta) (TokenPair, error) {

	account, err := s.accounts.FindByIdentifier(ctx, validate.NormalizeIdentifier(identifier))
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

// recentSessionLimit bounds the security center's session list — enough to
// show every device anyone plausibly still has logged in, plus recent
// history, without this becoming an unbounded query. Matches
// DefaultPageSize-scale limits used elsewhere in this codebase (ADR-0004).
const recentSessionLimit = 50

// ListSessions returns the caller's own recent sessions, newest first —
// both currently active ones and revoked/expired history, which doubles as
// the "登录记录" the security center shows: each row already is one login.
func (s *Service) ListSessions(ctx context.Context) ([]Session, error) {
	principal, ok := PrincipalFrom(ctx)
	if !ok {
		return nil, ErrMissingCredentials()
	}
	return s.sessions.ListRecentByUser(ctx, principal.UserID, recentSessionLimit)
}

// RevokeSession revokes one of the caller's own sessions by id.
//
// "Not found" and "not yours" are reported identically — see
// SessionRepository.RevokeOwned's doc comment — and so is "already
// revoked": all three answer with the same not-found error, because from
// the caller's point of view every one of them means the same thing, "this
// session is not something you can revoke right now."
func (s *Service) RevokeSession(ctx context.Context, id FamilyID) error {
	principal, ok := PrincipalFrom(ctx)
	if !ok {
		return ErrMissingCredentials()
	}
	revoked, err := s.sessions.RevokeOwned(ctx, id, principal.UserID)
	if err != nil {
		return err
	}
	if !revoked {
		return apperr.NotFound("SESSION_NOT_FOUND", "session not found")
	}
	return s.audit.Record(ctx, AuditEntry{
		ActorID: &principal.UserID, Action: "session.revoked",
		TargetType: "user", TargetID: principal.UserID,
		After: map[string]string{"session_id": id.String()},
	})
}

// RevokeOtherSessions revokes every one of the caller's own sessions except
// the one the current request is authenticated through — "log out
// everywhere else."
func (s *Service) RevokeOtherSessions(ctx context.Context) (int64, error) {
	principal, ok := PrincipalFrom(ctx)
	if !ok {
		return 0, ErrMissingCredentials()
	}
	family, err := ParseFamilyID(principal.TokenID)
	if err != nil {
		// The access token's jti is always a family id as of this
		// version (see issuePair) — a token that fails to parse as one
		// was not issued by this service, and Authenticate would already
		// have rejected it. Treated as an internal error rather than a
		// client one: nothing the caller did wrong produced this.
		return 0, apperr.Internal(fmt.Errorf("revoke other sessions: parse current session id: %w", err))
	}
	revoked, err := s.sessions.RevokeAllForUserExceptFamily(ctx, principal.UserID, family)
	if err != nil {
		return 0, err
	}
	if err := s.audit.Record(ctx, AuditEntry{
		ActorID: &principal.UserID, Action: "session.revoked_others",
		TargetType: "user", TargetID: principal.UserID,
		After: map[string]int64{"sessions_revoked": revoked},
	}); err != nil {
		return 0, err
	}
	return revoked, nil
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
		Issuer:   s.cfg.Issuer,
		Subject:  userID,
		Audience: s.cfg.Audience,
		IssuedAt: now,
		// TokenID (the jti) is the session's family id, not a fresh,
		// otherwise-unused identifier — every access token minted from
		// the same login lineage (across refreshes) carries the same
		// value. That is what lets Principal.TokenID answer "which
		// session is this request authenticated through" for the
		// security center's session list to mark as current, with no
		// extra claim and no extra round trip — see authhttp's
		// sessionResponse.
		ExpiresAt: now.Add(s.cfg.AccessTTL),
		TokenID:   family.String(),
		Role:      role,
	})
	if err != nil {
		return TokenPair{}, apperr.Internal(err)
	}

	return TokenPair{
		AccessToken:  access,
		RefreshToken: refreshToken,
		ExpiresIn:    int(s.cfg.AccessTTL.Seconds()),
		SessionID:    family.String(),
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
