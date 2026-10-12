// Package invite holds the member-invitation domain: the invite lifecycle
// (create, list, revoke, reinvite, accept) and the rules around it. It knows
// nothing about HTTP or SQL — those live in invitehttp and invitepg.
package invite

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/validate"
)

// Status is the invitation's place in its one-way lifecycle:
// pending -> accepted | expired | revoked. There is no path back to pending —
// a lapsed or withdrawn invitation is reinvited, which is a new row (see
// Service.Reinvite), not a resurrected old one; that keeps each row's history
// unambiguous for the audit trail.
type Status string

// The closed set of statuses.
const (
	StatusPending  Status = "pending"
	StatusAccepted Status = "accepted"
	StatusExpired  Status = "expired"
	StatusRevoked  Status = "revoked"
)

// Invitation is the domain entity. The token itself never appears here — see
// Repository's doc comment — only whether one still exists to be redeemed.
type Invitation struct {
	ID             string
	Email          string
	DepartmentID   *string
	Role           auth.Role
	Status         Status
	InvitedBy      string
	AcceptedUserID *string
	ExpiresAt      time.Time
	AcceptedAt     *time.Time
	RevokedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Expired reports whether a still-pending invitation's clock has run out.
// Expiry is not a status transition an outside caller ever announces — it is
// discovered lazily, the first time something looks at the row after the
// deadline — so this is the check that discovery uses. See
// Service.markExpiredIfDue.
func (i Invitation) Expired(now time.Time) bool {
	return i.Status == StatusPending && now.After(i.ExpiresAt)
}

// NewID mints a UUIDv7 for a new invitation, matching user.NewID (ADR-0006).
func NewID() string { return uuid.Must(uuid.NewV7()).String() }

// ListFilter narrows ListFiltered. A nil/empty field means "no constraint on
// this dimension."
type ListFilter struct {
	Status *Status
	Email  string
}

// Repository is the persistence port.
//
// It never sees a plaintext token: Create and GetByTokenHash both take the
// SHA-256 digest computed by the service (auth.HashRefreshToken — the same
// mechanism refresh tokens use, reused rather than duplicated, since the
// requirement is identical: an opaque bearer credential a database leak must
// not yield). The token generated at Create time is returned to the caller
// once, over the same call, and never persisted.
type Repository interface {
	Create(ctx context.Context, inv Invitation, tokenHash []byte) (Invitation, error)
	GetByID(ctx context.Context, id string) (Invitation, error)
	// GetByTokenHash is a plain read — the actual one-time-use guarantee is
	// MarkAccepted's conditional UPDATE, not a lock taken here. Two
	// concurrent Accept calls for the same token can both read the row as
	// pending; only one of their MarkAccepted calls will find it still
	// pending by the time it runs, because PostgreSQL serialises the two
	// UPDATEs and the second re-evaluates its WHERE clause against the
	// first's committed result (the same pattern RevokeRefreshSession and
	// dead-letter replay already rely on).
	GetByTokenHash(ctx context.Context, tokenHash []byte) (Invitation, error)
	ListFiltered(ctx context.Context, f ListFilter, limit, offset int32) ([]Invitation, error)
	CountFiltered(ctx context.Context, f ListFilter) (int64, error)
	// MarkAccepted is the one-time-use guard: it only succeeds against a row
	// still in StatusPending, so a replay of an old link or a race between
	// two accepts of the same token fails on whichever loses the race
	// instead of provisioning two accounts.
	MarkAccepted(ctx context.Context, id, acceptedUserID string) error
	MarkExpired(ctx context.Context, id string) error
	Revoke(ctx context.Context, id string) error
}

// TxManager runs a function inside a database transaction (ADR-0005).
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// EventPublisher records a domain event — declared here rather than
// depending on messaging.Publisher directly, matching user.EventPublisher's
// relationship to messaging.Publisher.
type EventPublisher interface {
	Publish(ctx context.Context, event messaging.Event) error
}

// AuditRecorder records a management action — matching user.AuditRecorder's
// relationship to the audit package.
type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry) error
}

// PasswordHasher is the hashing port Accept uses for the member's
// self-chosen password. Declared here rather than shared with user's
// identically-shaped interface, for the same reason EventPublisher and
// AuditRecorder are each declared locally: the caller defines the interface
// it needs (ADR-0002).
type PasswordHasher interface {
	Hash(password string) (string, error)
}

// MemberProvisioner creates the account an accepted invitation grants.
//
// It returns only the new member's ID, not a user.User: this package must
// not import package user (that would make user -> invite -> user a cycle,
// since a future user.Service method satisfying this interface would need
// to import invite for ProvisionedMember's type — see the adapter that
// actually implements this in cmd/api, which is the one place allowed to
// import everything).
type MemberProvisioner interface {
	Provision(ctx context.Context, in ProvisionedMember) (memberID string, err error)
}

// ProvisionedMember is the validated input to MemberProvisioner.Provision.
type ProvisionedMember struct {
	Email        string
	Name         string
	PasswordHash string
	Role         auth.Role
	DepartmentID *string
}

// Clock is injected so that expiry can be tested without sleeping — matching
// auth.Clock.
type Clock func() time.Time

// EventInvitationCreated is emitted when an invitation is issued (initial or
// reinvite). It is the event package notify sends the invitation email for.
const EventInvitationCreated = "invitation.created.v1"

// InvitationCreatedPayload is the body of EventInvitationCreated.
//
// Token is the AES-256-GCM encryption of the plaintext (TokenCipher.Encrypt),
// not the plaintext itself. The only consumer that exists (notify.
// InvitationHandler) needs the real value to put in a link, and there is
// nowhere durable for that value to live in the clear — see Repository's
// doc comment for why it is never written back to the invitations table —
// but "nowhere to persist it" is not the same claim as "safe to persist
// anyway in the outbox". outbox_events is a real table, read by the
// messaging admin endpoints, mirrored into any configured broker, and
// liable to end up in dead_letter_events if delivery fails; every one of
// those is a place a bearer credential good for creating an account should
// not sit in the clear. Encrypting it here means a reader of any of those —
// including an operator debugging the outbox, or an attacker who gains
// read access to it without gaining the application's key — sees
// ciphertext, not a redeemable token. Exposure is bounded by the same
// outbox retention/pruning window every other event already has
// (MESSAGING_PROCESSED_RETENTION, PruneInterval); no separate expiry policy
// is needed on top of that, since a redelivered or retained event is only
// as sensitive as the invitation itself, which independently expires.
type InvitationCreatedPayload struct {
	InvitationID string `json:"invitation_id"`
	Email        string `json:"email"`
	Token        string `json:"token"`
}

// TokenEncrypter encrypts the token this package puts on the wire. Declared
// as an interface here, even though *TokenCipher (this package's own type)
// is the only implementation anyone wires up, so the service's tests can
// substitute a no-op or a spy instead of exercising real AES-GCM.
type TokenEncrypter interface {
	Encrypt(plaintext string) (string, error)
}

// Service holds the invite use cases.
type Service struct {
	repo        Repository
	tx          TxManager
	events      EventPublisher
	audit       AuditRecorder
	hasher      PasswordHasher
	provisioner MemberProvisioner
	cipher      TokenEncrypter
	ttl         time.Duration
	now         Clock
}

// NewService wires the service. ttl is the default validity period for a
// newly created invitation (config.Invite.TTL). cipher encrypts the token
// before it is queued for delivery — see InvitationCreatedPayload's doc
// comment; cmd/worker constructs the matching TokenCipher to decrypt it.
func NewService(
	repo Repository, tx TxManager, events EventPublisher, auditRecorder AuditRecorder,
	hasher PasswordHasher, provisioner MemberProvisioner, cipher TokenEncrypter, ttl time.Duration,
) *Service {
	return &Service{
		repo: repo, tx: tx, events: events, audit: auditRecorder,
		hasher: hasher, provisioner: provisioner, cipher: cipher, ttl: ttl, now: time.Now,
	}
}

// SetClock overrides the clock, for tests.
func (s *Service) SetClock(c Clock) {
	if c == nil {
		c = time.Now
	}
	s.now = c
}

// NewInput is the validated input for Invite and Reinvite.
type NewInput struct {
	Email        string
	DepartmentID *string
	Role         auth.Role
}

// Invite issues a new invitation and queues its email.
//
// The plaintext token is returned once, directly to the caller, in addition
// to being queued for email delivery. That is deliberate, not a leak: the
// caller already holds `invite:create`, the same trust level that lets
// cmd/adminctl print a freshly generated password to its own operator once
// (see bootstrapContext's reasoning), and this project's only email
// implementation is a development placeholder that logs rather than sends
// (package notify's LogSender) — without handing the token back here, an
// invitation would be unredeemable in exactly the environment this scaffold
// ships for. A production deployment with a real mail provider can simply
// have its frontend not display the value.
func (s *Service) Invite(ctx context.Context, in NewInput) (Invitation, string, error) {
	if err := auth.Authorize(ctx, auth.PermInviteCreate); err != nil {
		return Invitation{}, "", err
	}
	return s.create(ctx, in, "member.invited")
}

func (s *Service) create(ctx context.Context, in NewInput, action string) (Invitation, string, error) {
	email := validate.NormalizeEmail(in.Email)
	var v validate.Violations
	v.Required("email", email)
	v.Email("email", email)
	if !in.Role.Valid() {
		v.Add("role", "must be one of the defined roles")
	}
	if v.Any() {
		return Invitation{}, "", v.Err("INVALID_ARGUMENT", "invalid invitation")
	}
	// Both Invite and Reinvite funnel through here, which is what makes this
	// the one place that needs to refuse a non-system-administrator issuing
	// (or reissuing, on reinvite) an invitation carrying the system_admin
	// role — invite:create is held by member_admin too, and without this it
	// would be able to mint a system administrator account by invitation
	// instead of by direct role assignment. See auth.GuardRoleGrant.
	if err := auth.GuardRoleGrant(ctx, in.Role); err != nil {
		return Invitation{}, "", err
	}

	token, tokenHash, err := auth.NewRefreshToken()
	if err != nil {
		return Invitation{}, "", apperr.Internal(err)
	}

	var created Invitation
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		principal, _ := auth.PrincipalFrom(ctx)
		var err error
		created, err = s.repo.Create(ctx, Invitation{
			ID: NewID(), Email: email, DepartmentID: in.DepartmentID, Role: in.Role,
			Status: StatusPending, InvitedBy: principal.UserID, ExpiresAt: s.now().Add(s.ttl),
		}, tokenHash)
		if err != nil {
			return err
		}

		encryptedToken, err := s.cipher.Encrypt(token)
		if err != nil {
			return apperr.Internal(err)
		}
		event, err := messaging.NewEvent(EventInvitationCreated, InvitationCreatedPayload{
			InvitationID: created.ID, Email: created.Email, Token: encryptedToken,
		})
		if err != nil {
			return apperr.Internal(err)
		}
		event.TraceParent = messaging.TraceParentFromContext(ctx)
		if err := s.events.Publish(ctx, event); err != nil {
			return apperr.Internal(err)
		}

		return s.audit.Record(ctx, audit.Entry{
			ActorID: actorID(ctx), Action: action, TargetType: "invitation", TargetID: created.ID,
			Result: audit.ResultSuccess,
			After:  map[string]any{"email": created.Email, "role": created.Role.String(), "expires_at": created.ExpiresAt},
		})
	})
	if err != nil {
		return Invitation{}, "", err
	}
	return created, token, nil
}

// Get returns one invitation.
func (s *Service) Get(ctx context.Context, id string) (Invitation, error) {
	if err := auth.Authorize(ctx, auth.PermInviteRead); err != nil {
		return Invitation{}, err
	}
	inv, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Invitation{}, err
	}
	return s.markExpiredIfDue(ctx, inv)
}

// Page is one slice of a listing together with the total a client needs to
// render pagination controls.
type Page struct {
	Items      []Invitation
	Page       int
	PageSize   int
	Total      int64
	TotalPages int
}

// Pagination bounds, matching user.Service's (ADR-0004).
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
	maxOffset       = 1 << 30
)

// List returns a filtered, paginated page of invitations.
func (s *Service) List(ctx context.Context, f ListFilter, page, pageSize int) (Page, error) {
	if err := auth.Authorize(ctx, auth.PermInviteRead); err != nil {
		return Page{}, err
	}
	if page < 1 {
		page = 1
	}
	switch {
	case pageSize < 1:
		pageSize = DefaultPageSize
	case pageSize > MaxPageSize:
		pageSize = MaxPageSize
	}

	total, err := s.repo.CountFiltered(ctx, f)
	if err != nil {
		return Page{}, err
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if page > maxOffset/pageSize {
		return Page{Items: []Invitation{}, Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}, nil
	}
	offset := int32((page - 1) * pageSize) //nolint:gosec // bounded above
	items, err := s.repo.ListFiltered(ctx, f, int32(pageSize), offset)
	if err != nil {
		return Page{}, err
	}
	return Page{Items: items, Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}, nil
}

// Revoke withdraws a pending invitation. Like MarkAccepted, the underlying
// update only succeeds against a still-pending row, so revoking one that was
// already accepted, expired or revoked fails with a conflict rather than
// silently doing nothing.
func (s *Service) Revoke(ctx context.Context, id string) error {
	if err := auth.Authorize(ctx, auth.PermInviteRevoke); err != nil {
		return err
	}
	inv, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	inv, err = s.markExpiredIfDue(ctx, inv)
	if err != nil {
		return err
	}
	if inv.Status != StatusPending {
		return apperr.Conflict("INVITATION_NOT_PENDING", "this invitation is no longer pending")
	}

	return s.tx.WithTx(ctx, func(ctx context.Context) error {
		return s.revokeLocked(ctx, inv)
	})
}

// revokeLocked marks a pending invitation revoked and records its audit
// entry. It must be called from inside an already-open transaction — Revoke
// and Reinvite are its only two callers, and each opens one, Reinvite's
// wrapping the subsequent create() as well so the two actions ("revoke the
// old one", "issue the new one") commit or roll back together. Factoring
// this out is what lets both callers produce the identical
// "invitation.revoked" audit record instead of Reinvite's revoke step
// silently skipping it, which is what let the old and new invitations'
// audit trail go out of sync with what Revoke itself produces.
func (s *Service) revokeLocked(ctx context.Context, inv Invitation) error {
	if err := s.repo.Revoke(ctx, inv.ID); err != nil {
		return err
	}
	return s.audit.Record(ctx, audit.Entry{
		ActorID: actorID(ctx), Action: "invitation.revoked", TargetType: "invitation", TargetID: inv.ID,
		Result: audit.ResultSuccess, Before: map[string]any{"email": inv.Email},
	})
}

// Reinvite revokes an outstanding invitation to the same email, if any, and
// issues a new one with a fresh token and expiry. It is implemented as
// revoke-then-create — a new row — rather than reusing the old one, so each
// row's lifecycle stays the simple one-way state machine Status documents,
// and the audit trail sees "revoked, then invited again" rather than a
// mutation that erases what the row previously said.
func (s *Service) Reinvite(ctx context.Context, id string) (Invitation, string, error) {
	if err := auth.Authorize(ctx, auth.PermInviteCreate); err != nil {
		return Invitation{}, "", err
	}
	prior, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Invitation{}, "", err
	}
	prior, err = s.markExpiredIfDue(ctx, prior)
	if err != nil {
		return Invitation{}, "", err
	}

	// The old invitation's revocation and the new one's creation (insert,
	// outbox event, audit record) commit or roll back as one unit: opening
	// the transaction here, before either step, and letting create()'s own
	// s.tx.WithTx detect and reuse it (ADR-0005) rather than starting a
	// nested one, is what closes the gap the old implementation left — a
	// failure partway through create() used to leave the prior invitation
	// revoked with no replacement, silently invalidating a link that had
	// just been handed out.
	var created Invitation
	var token string
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		if prior.Status == StatusPending {
			if err := s.revokeLocked(ctx, prior); err != nil {
				return err
			}
		}
		var err error
		created, token, err = s.create(ctx, NewInput{Email: prior.Email, DepartmentID: prior.DepartmentID, Role: prior.Role}, "member.reinvited")
		return err
	})
	if err != nil {
		return Invitation{}, "", err
	}
	return created, token, nil
}

// AcceptInput is the validated input for Accept.
type AcceptInput struct {
	Token    string
	Name     string
	Password string
}

// Accept redeems a one-time invitation token and provisions the member's
// account, self-chosen password included.
//
// This is the one method in the package with no auth.Authorize call: the
// caller holds no principal — accepting an invitation is how an account
// comes to exist, not something an existing one does — so the token itself
// is the authorization, verified by GetByTokenHash finding a still-pending,
// unexpired row.
func (s *Service) Accept(ctx context.Context, in AcceptInput) (memberID string, err error) {
	tokenHash := auth.HashRefreshToken(in.Token)
	inv, err := s.repo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return "", err
	}
	inv, err = s.markExpiredIfDue(ctx, inv)
	if err != nil {
		return "", err
	}
	if inv.Status != StatusPending {
		return "", apperr.Conflict("INVITATION_NOT_PENDING",
			"this invitation has already been used, expired, or was revoked")
	}

	var v validate.Violations
	v.Required("name", in.Name)
	v.RuneLen("name", in.Name, 1, 100)
	v.NoControlChars("name", in.Name)
	if v.Any() {
		return "", v.Err("INVALID_ARGUMENT", "invalid acceptance")
	}
	if err := auth.ValidatePassword(in.Password); err != nil {
		return "", err
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return "", apperr.Internal(err)
	}

	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		memberID, err = s.provisioner.Provision(ctx, ProvisionedMember{
			Email: inv.Email, Name: in.Name, PasswordHash: hash, Role: inv.Role, DepartmentID: inv.DepartmentID,
		})
		if err != nil {
			return err
		}
		if err := s.repo.MarkAccepted(ctx, inv.ID, memberID); err != nil {
			return err
		}
		id := memberID
		return s.audit.Record(ctx, audit.Entry{
			ActorID: &id, Action: "member.invite_accepted", TargetType: "user", TargetID: memberID,
			Result: audit.ResultSuccess, After: map[string]any{"email": inv.Email, "role": inv.Role.String()},
		})
	})
	if err != nil {
		return "", err
	}
	return memberID, nil
}

// markExpiredIfDue transitions a pending-but-overdue invitation to expired
// and returns the updated view. It is called from every read and write path
// that touches a specific invitation, which is what makes expiry accurate
// without a background sweep: nothing observes a lapsed invitation as still
// pending, because the first thing to look at it after the deadline fixes
// the status before acting on it.
func (s *Service) markExpiredIfDue(ctx context.Context, inv Invitation) (Invitation, error) {
	if !inv.Expired(s.now()) {
		return inv, nil
	}
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		if err := s.repo.MarkExpired(ctx, inv.ID); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{
			Action: "invitation.expired", TargetType: "invitation", TargetID: inv.ID,
			Result: audit.ResultSuccess, Before: map[string]any{"email": inv.Email},
		})
	})
	if err != nil {
		return Invitation{}, err
	}
	inv.Status = StatusExpired
	return inv, nil
}

func actorID(ctx context.Context) *string {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil
	}
	id := p.UserID
	return &id
}
