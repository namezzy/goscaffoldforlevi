package user

import (
	"context"
	"log/slog"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/notification"
	"github.com/namezzy/goscaffoldforlevi/internal/validate"
)

// Repository is the persistence port.
//
// It is defined here, next to its consumer, and lists exactly the operations
// this service performs. A generic Repository[T] with Find/Save/Delete would
// hide which of those the service actually needs, force every implementation to
// support operations nobody calls, and make the compiler useless as a guide
// when a query has to change.
type Repository interface {
	Create(ctx context.Context, u User) (User, error)
	GetByID(ctx context.Context, id ID) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	// GetByIdentifier looks up a user by email or username — whichever the
	// caller's already-normalised identifier matches. Login is the only
	// caller; everything else in this service knows which kind of value it
	// has.
	GetByIdentifier(ctx context.Context, identifier string) (User, error)
	List(ctx context.Context, limit, offset int32) ([]User, error)
	Count(ctx context.Context) (int64, error)
	// ListFiltered and CountFiltered back the member directory search: a
	// substring match plus role/status filters and a sort order. They are
	// separate from List/Count, rather than List/Count growing optional
	// arguments, so that the unfiltered path stays a plain, obviously-correct
	// query.
	ListFiltered(ctx context.Context, f ListFilter, limit, offset int32) ([]User, error)
	CountFiltered(ctx context.Context, f ListFilter) (int64, error)
	Update(ctx context.Context, id ID, p Patch) (User, error)
	Delete(ctx context.Context, id ID) error
	// CountActiveByRole backs the "protect the last system admin" rule in
	// Update and Delete.
	CountActiveByRole(ctx context.Context, role auth.Role) (int64, error)
	// LockActiveByRole is CountActiveByRole's transactional sibling: called
	// from inside the write's own transaction, it locks the rows it counts
	// so a concurrent call for the same role blocks until this transaction
	// commits — see queries/user.sql's LockActiveUsersByRole for why that
	// lock, not just the count, is what the guard actually depends on.
	LockActiveByRole(ctx context.Context, role auth.Role) (int64, error)
}

// TxManager runs a function inside a database transaction. Declared here, as
// a one-method interface, so the service can make a write and its event atomic
// without importing a driver (ADR-0002).
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// EventPublisher records a domain event.
//
// The service does not know, and must not know, whether this reaches a broker.
// What it relies on is narrower and is the reason the call sits inside the
// transaction: an accepted event is durable exactly when the change that
// caused it is.
type EventPublisher interface {
	Publish(ctx context.Context, event messaging.Event) error
}

// PasswordHasher is the hashing port. Declaring it here rather than importing a
// concrete hasher keeps the service's tests fast: a real Argon2id hash costs
// 64 MiB and tens of milliseconds per call.
type PasswordHasher interface {
	Hash(password string) (string, error)
}

// AuditRecorder records a management action for the audit trail. Declared
// here rather than depending on audit.Recorder directly, the same
// relationship EventPublisher has to messaging.Publisher: the caller defines
// the interface it needs (ADR-0002).
type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry) error
}

type nopAuditRecorder struct{}

func (nopAuditRecorder) Record(context.Context, audit.Entry) error { return nil }

// SessionRevoker revokes every active session a user holds. Declared here for
// the same reason as AuditRecorder: Deactivate needs exactly this one
// operation, not auth's whole SessionRepository.
type SessionRevoker interface {
	RevokeAllForUser(ctx context.Context, userID string) (int64, error)
}

type nopSessionRevoker struct{}

func (nopSessionRevoker) RevokeAllForUser(context.Context, string) (int64, error) { return 0, nil }

// NotificationRecorder creates an in-app notification. Declared here for
// the same reason as AuditRecorder: Update needs exactly this one
// operation when a member's role changes — "与成员相关的重要事件触发通知" — not
// notification's whole read-side Service.
type NotificationRecorder interface {
	Notify(ctx context.Context, n notification.NewNotification) error
}

type nopNotificationRecorder struct{}

func (nopNotificationRecorder) Notify(context.Context, notification.NewNotification) error {
	return nil
}

// Pagination bounds (ADR-0004). The maximum exists so that a client cannot turn
// one request into a full table scan by asking for a page of a million.
const (
	DefaultPageSize = 20
	MaxPageSize     = 100

	// maxOffset is the largest OFFSET this service will issue. It is far above
	// any realistic table size and far below the int32 limit, which is what
	// makes the conversion in List provably safe.
	maxOffset = 1 << 30
)

// SortBy is a column ListFiltered may order by. The set is closed to exactly
// the columns userpg has a query plan for — see queries/user.sql — rather
// than accepting an arbitrary client-supplied column name.
type SortBy string

// The closed set of sortable columns.
const (
	SortByCreatedAt SortBy = "created_at"
	SortByName      SortBy = "name"
	SortByEmail     SortBy = "email"
)

// Valid reports whether s is one of the defined sort columns.
func (s SortBy) Valid() bool { return s == SortByCreatedAt || s == SortByName || s == SortByEmail }

// ListFilter narrows and orders ListFiltered. A nil/empty field means "no
// constraint on this dimension."
type ListFilter struct {
	Search   string
	Role     *auth.Role
	Status   *Status
	SortBy   SortBy
	SortDesc bool
}

// Service holds the user use cases.
type Service struct {
	repo   Repository
	hasher PasswordHasher
	tx     TxManager
	events EventPublisher
	log    *slog.Logger

	// observer, audit, sessions and notifications are never nil; see their
	// setters.
	observer      Observer
	audit         AuditRecorder
	sessions      SessionRevoker
	notifications NotificationRecorder
}

// NewService wires the service.
func NewService(
	repo Repository,
	hasher PasswordHasher,
	tx TxManager,
	events EventPublisher,
	log *slog.Logger,
) *Service {
	return &Service{repo: repo, hasher: hasher, tx: tx, events: events, log: log,
		observer: NopObserver(), audit: nopAuditRecorder{}, sessions: nopSessionRevoker{},
		notifications: nopNotificationRecorder{}}
}

// SetObserver attaches instrumentation. Passing nil restores the no-op, so a
// caller that has not configured metrics cannot leave a nil interface on the
// request path.
func (s *Service) SetObserver(o Observer) {
	if o == nil {
		o = NopObserver()
	}
	s.observer = o
}

// SetAuditRecorder attaches the audit trail. Passing nil restores the no-op.
func (s *Service) SetAuditRecorder(a AuditRecorder) {
	if a == nil {
		a = nopAuditRecorder{}
	}
	s.audit = a
}

// SetSessionRevoker attaches session revocation, used when a member is
// deactivated. Passing nil restores the no-op.
func (s *Service) SetSessionRevoker(r SessionRevoker) {
	if r == nil {
		r = nopSessionRevoker{}
	}
	s.sessions = r
}

// SetNotificationRecorder attaches in-app notification delivery, used when
// a member's role changes. Passing nil restores the no-op.
func (s *Service) SetNotificationRecorder(n NotificationRecorder) {
	if n == nil {
		n = nopNotificationRecorder{}
	}
	s.notifications = n
}

// CreateInput is the validated input for registration.
type CreateInput struct {
	Email    string
	Username string
	Name     string
	Password string
	Role     auth.Role
}

// Create registers a user.
//
// Uniqueness is not pre-checked with a SELECT. Between the check and the insert
// there is a window in which another request can insert the same email, and
// under concurrent signup it is reliably lost. The unique index decides, and
// the repository turns its violation into a conflict.
func (s *Service) Create(ctx context.Context, in CreateInput) (User, error) {
	email := validate.NormalizeEmail(in.Email)
	// A username is optional at creation, so it is only normalised and
	// validated when the caller actually supplied one.
	var username string
	if in.Username != "" {
		username = validate.NormalizeUsername(in.Username)
	}

	var v validate.Violations
	v.Required("email", email)
	v.Email("email", email)
	if username != "" {
		v.Username("username", username)
	}
	v.Required("name", in.Name)
	v.RuneLen("name", in.Name, 1, 100)
	v.NoControlChars("name", in.Name)
	if !in.Role.Valid() {
		v.Add("role", "must be one of: system_admin, member_admin, operator, read_only, auditor")
	}
	if v.Any() {
		return User{}, v.Err("INVALID_ARGUMENT", "invalid user")
	}
	if err := auth.GuardRoleGrant(ctx, in.Role); err != nil {
		return User{}, err
	}
	// The password is validated by the hasher, which owns the policy, but it is
	// checked before hashing so that an over-long password is rejected without
	// paying for the hash.
	if err := auth.ValidatePassword(in.Password); err != nil {
		return User{}, err
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return User{}, apperr.Internal(err)
	}

	// The insert, the event and the audit record are one transaction.
	// Publishing/recording after the commit would lose them whenever the
	// process dies in between, and it would leave no evidence that it had:
	// the user exists, nothing downstream ever hears about it, and only a
	// reconciliation job would ever notice.
	var created User
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		created, err = s.repo.Create(ctx, User{
			ID:           NewID(),
			Email:        email,
			Username:     username,
			PasswordHash: hash,
			Name:         in.Name,
			Status:       StatusActive,
			Role:         in.Role,
		})
		if err != nil {
			return err
		}
		if err := s.publishCreated(ctx, created); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{
			ActorID: actorID(ctx), Action: "member.created", TargetType: "user", TargetID: created.ID.String(),
			Result: audit.ResultSuccess,
			After:  map[string]any{"email": created.Email, "name": created.Name, "role": created.Role.String()},
		})
	})
	if err != nil {
		return User{}, err
	}

	// Counted after the commit, so the number matches the users table. Counting
	// the attempt would make the signup graph a graph of form submissions, and
	// a validation regression that rejected every signup would read as record
	// traffic rather than as an outage.
	s.observer.UserRegistered()

	return created, nil
}

// EventUserCreated is emitted when a user is registered.
const EventUserCreated = "user.created.v1"

// CreatedPayload is the body of EventUserCreated.
//
// It carries an email because the only consumer that exists needs to send mail
// to it, and it carries nothing else that a consumer could get by asking. No
// password, no hash, no token: an event is copied into logs, into a broker,
// and into every consumer's storage, so anything secret in it has been leaked
// several times over before anyone reviews it. ADR-0009 makes this a rule
// rather than a habit.
type CreatedPayload struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}

func (s *Service) publishCreated(ctx context.Context, u User) error {
	event, err := messaging.NewEvent(EventUserCreated, CreatedPayload{
		UserID: u.ID.String(),
		Email:  u.Email,
		Name:   u.Name,
		Role:   string(u.Role),
	})
	if err != nil {
		return apperr.Internal(err)
	}
	event.TraceParent = messaging.TraceParentFromContext(ctx)

	if err := s.events.Publish(ctx, event); err != nil {
		// Deliberately fatal to the request. The event is part of the write:
		// letting the user be created without it is exactly the silent
		// inconsistency the outbox exists to prevent, and a 500 on a failure
		// this rare is cheaper than an unnoticed divergence.
		return apperr.Internal(err)
	}
	return nil
}

// ProvisionInput is the validated input for creating a member from an
// accepted invitation.
type ProvisionInput struct {
	Email        string
	Name         string
	PasswordHash string
	Role         auth.Role
	DepartmentID *string
}

// ProvisionFromInvite creates the account an accepted invitation grants.
//
// It skips the PermUserCreate check every other write in this service makes:
// the caller holds no principal at all — invite.Service.Accept runs before
// the new account exists to be one — and its authorization is a different
// mechanism entirely, possession of the one-time invitation token, checked
// by invite.Service before this is ever called. This method exists
// specifically for that caller (mirrors cmd/adminctl's bootstrapContext,
// which documents the same reasoning for the same kind of exception).
//
// It does not open its own transaction: the caller already has one open (its
// own WithTx), and s.tx.WithTx joins that existing transaction rather than
// starting a nested one — see docs/adr/0005-data-access.md.
func (s *Service) ProvisionFromInvite(ctx context.Context, in ProvisionInput) (User, error) {
	var created User
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		created, err = s.repo.Create(ctx, User{
			ID: NewID(), Email: in.Email, PasswordHash: in.PasswordHash,
			Name: in.Name, Status: StatusActive, Role: in.Role, DepartmentID: in.DepartmentID,
		})
		if err != nil {
			return err
		}
		return s.publishCreated(ctx, created)
	})
	if err != nil {
		return User{}, err
	}
	s.observer.UserRegistered()
	return created, nil
}

// Get returns one user.
//
// Authorization is checked here rather than in middleware because it depends on
// who the record belongs to, and that is only knowable from the request's
// target — see auth.AuthorizeSelfOrPermission.
func (s *Service) Get(ctx context.Context, id ID) (User, error) {
	if err := auth.AuthorizeSelfOrPermission(ctx, id.String(), auth.PermUserRead); err != nil {
		return User{}, err
	}
	return s.repo.GetByID(ctx, id)
}

// List returns a page of users. There is no self-service variant: listing is an
// administrative capability, so it is a plain permission check.
func (s *Service) List(ctx context.Context, page, pageSize int) (Page, error) {
	if err := auth.Authorize(ctx, auth.PermUserList); err != nil {
		return Page{}, err
	}

	page, pageSize = normalizePaging(page, pageSize)

	total, err := s.repo.Count(ctx)
	if err != nil {
		return Page{}, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	// The conversions below are safe because both values have just been
	// clamped: pageSize is at most MaxPageSize, and page is bounded so that the
	// offset cannot exceed maxOffset.
	//
	// The bound is checked by division rather than by comparing
	// page*pageSize against the limit. That multiplication overflows for a
	// large page — page=math.MaxInt with pageSize=20 wraps to -20, which is
	// below any limit — so the guard against the overflow was itself the
	// overflow, and a negative OFFSET reached PostgreSQL. Dividing cannot
	// overflow, because pageSize is at least 1 here.
	if page > maxOffset/pageSize {
		return Page{
			Items: []User{}, Page: page, PageSize: pageSize,
			Total: total, TotalPages: totalPages,
		}, nil
	}
	offset := int32((page - 1) * pageSize)                  //nolint:gosec // bounded above
	items, err := s.repo.List(ctx, int32(pageSize), offset) //nolint:gosec // <= MaxPageSize
	if err != nil {
		return Page{}, err
	}

	return Page{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

// ListFiltered returns a page of the member directory: a substring search
// plus role/status filters and a choice of sort order. Like List, this is an
// administrative capability with no self-service variant.
func (s *Service) ListFiltered(ctx context.Context, f ListFilter, page, pageSize int) (Page, error) {
	if err := auth.Authorize(ctx, auth.PermUserList); err != nil {
		return Page{}, err
	}
	if f.Role != nil && !f.Role.Valid() {
		return Page{}, apperr.Invalid("INVALID_QUERY_PARAM", "role must be one of the defined roles",
			apperr.Detail{Field: "role", Reason: "not a defined role"})
	}
	if f.Status != nil && !f.Status.Valid() {
		return Page{}, apperr.Invalid("INVALID_QUERY_PARAM", "status must be one of: active, disabled",
			apperr.Detail{Field: "status", Reason: "not a defined status"})
	}
	if f.SortBy == "" {
		f.SortBy = SortByCreatedAt
	}
	if !f.SortBy.Valid() {
		return Page{}, apperr.Invalid("INVALID_QUERY_PARAM", "sort must be one of: created_at, name, email",
			apperr.Detail{Field: "sort", Reason: "not a sortable column"})
	}

	page, pageSize = normalizePaging(page, pageSize)

	total, err := s.repo.CountFiltered(ctx, f)
	if err != nil {
		return Page{}, err
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	if page > maxOffset/pageSize {
		return Page{Items: []User{}, Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}, nil
	}
	offset := int32((page - 1) * pageSize) //nolint:gosec // bounded above
	items, err := s.repo.ListFiltered(ctx, f, int32(pageSize), offset)
	if err != nil {
		return Page{}, err
	}

	return Page{Items: items, Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}, nil
}

// normalizePaging clamps client input instead of rejecting it. A page size of
// 1000 is a reasonable thing to ask for and an unreasonable thing to serve; an
// error would be pedantic, while silently serving it would be a denial of
// service.
func normalizePaging(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	switch {
	case pageSize < 1:
		pageSize = DefaultPageSize
	case pageSize > MaxPageSize:
		pageSize = MaxPageSize
	}
	return page, pageSize
}

// Update applies a partial update.
func (s *Service) Update(ctx context.Context, id ID, p Patch) (User, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return User{}, auth.ErrMissingCredentials()
	}
	actingOnSelf := principal.UserID == id.String()

	// Three tiers of authorization, from most to least sensitive:
	//   - a role change requires PermRoleAssign specifically, not just "may
	//     edit users" — see auth.PermRoleAssign's doc comment — and
	//     GuardRoleGrant on top of it: PermRoleAssign lets member_admin
	//     assign the four ordinary roles, never system_admin.
	//   - a status or department change requires PermUserUpdate: it is an
	//     administrative action on someone else's standing, never something
	//     ownership alone can authorise. Department is included here, not
	//     left to the self-service tier below: choosing your own department
	//     is org-management, not a personal-profile edit.
	//   - everything else (name, email, username) may be done by the owner
	//     on their own record, or by anyone holding PermUserUpdate on any
	//     record.
	switch {
	case p.Role != nil:
		if err := auth.Authorize(ctx, auth.PermRoleAssign); err != nil {
			return User{}, err
		}
		if err := auth.GuardRoleGrant(ctx, *p.Role); err != nil {
			return User{}, err
		}
	case p.ChangesPrivileges():
		if err := auth.Authorize(ctx, auth.PermUserUpdate); err != nil {
			return User{}, err
		}
	default:
		if err := auth.AuthorizeSelfOrPermission(ctx, id.String(), auth.PermUserUpdate); err != nil {
			return User{}, err
		}
	}

	if p.IsEmpty() {
		return User{}, apperr.Invalid("EMPTY_PATCH", "the request changes nothing")
	}

	var v validate.Violations
	if p.Email != nil {
		normalized := validate.NormalizeEmail(*p.Email)
		v.Email("email", normalized)
		p.Email = &normalized
	}
	if p.Username != nil {
		normalized := validate.NormalizeUsername(*p.Username)
		v.Username("username", normalized)
		p.Username = &normalized
	}
	if p.Name != nil {
		v.Required("name", *p.Name)
		v.RuneLen("name", *p.Name, 1, 100)
		v.NoControlChars("name", *p.Name)
	}
	if p.Status != nil && !p.Status.Valid() {
		v.Add("status", "must be one of: active, disabled")
	}
	if p.Role != nil && !p.Role.Valid() {
		v.Add("role", "must be one of: system_admin, member_admin, operator, read_only, auditor")
	}
	if v.Any() {
		return User{}, v.Err("INVALID_ARGUMENT", "invalid user patch")
	}

	// Fetched whenever the caller might be reaching into someone else's
	// account at all — needed for GuardActionOnAccount just below — or
	// whenever the change could touch the last-system-admin invariant.
	// Every other patch (a self-service edit of your own name, say) has no
	// need for it, and loading the row unconditionally would cost every one
	// of those a query to protect a check that only applies to admin
	// actions on someone else.
	var before User
	if !actingOnSelf || p.ChangesPrivileges() {
		var err error
		before, err = s.repo.GetByID(ctx, id)
		if err != nil {
			return User{}, err
		}
		if !actingOnSelf {
			if err := auth.GuardActionOnAccount(ctx, id.String(), before.Role); err != nil {
				return User{}, err
			}
		}
	}

	// Uniqueness is not pre-checked with a SELECT, for the same reason as
	// Create: the unique index decides, and the repository turns a violation
	// into EMAIL_ALREADY_EXISTS or USERNAME_ALREADY_EXISTS.
	var updated User
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		// The last-system-admin check runs here, inside this transaction,
		// not before it — see guardLastSystemAdminLocked's doc comment for
		// why a check made before the transaction opens cannot be made safe
		// under concurrency no matter how fresh its read was.
		demotesRole := p.Role != nil && *p.Role != auth.RoleSystemAdmin
		disables := p.Status != nil && *p.Status == StatusDisabled
		if before.Role == auth.RoleSystemAdmin && (demotesRole || disables) {
			if err := s.guardLastSystemAdminLocked(ctx); err != nil {
				return err
			}
		}

		var err error
		updated, err = s.repo.Update(ctx, id, p)
		if err != nil {
			return err
		}
		if err := s.recordUpdateAudit(ctx, id, before, updated, p); err != nil {
			return err
		}
		if p.Role != nil {
			if err := s.notifications.Notify(ctx, notification.NewNotification{
				UserID: id.String(), Type: "role_changed", Title: "Your role has changed",
				Body: "Your account role is now " + updated.Role.String(), TargetType: "user", TargetID: id.String(),
				DedupKey: "user:" + id.String() + ":role_changed:" + updated.Role.String() + ":" + updated.UpdatedAt.String(),
			}); err != nil {
				return err
			}
		}
		// A transition into disabled revokes every session the member holds,
		// so a deactivation takes effect immediately rather than at next
		// natural token expiry — see SessionRevoker's doc comment and
		// docs/architecture.md §10 for the access-token side of this: the
		// 15-minute access token itself is not revocable and keeps working
		// until it expires on its own, which the API response documents so
		// this is not a silent gap.
		if p.Status != nil && *p.Status == StatusDisabled {
			if _, err := s.sessions.RevokeAllForUser(ctx, id.String()); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return User{}, err
	}
	return updated, nil
}

func (s *Service) recordUpdateAudit(ctx context.Context, id ID, before, after User, p Patch) error {
	action := "member.updated"
	beforeSnap := map[string]any{}
	afterSnap := map[string]any{}
	if p.Role != nil {
		action = "member.role_changed"
		beforeSnap["role"] = before.Role.String()
		afterSnap["role"] = after.Role.String()
	}
	if p.Status != nil {
		switch {
		case *p.Status == StatusDisabled:
			action = "member.deactivated"
		case before.Status == StatusDisabled:
			action = "member.reactivated"
		}
		beforeSnap["status"] = before.Status.String()
		afterSnap["status"] = after.Status.String()
	}
	if p.Name != nil {
		afterSnap["name"] = after.Name
	}
	if p.Email != nil {
		afterSnap["email"] = after.Email
	}
	if p.Username != nil {
		afterSnap["username"] = after.Username
	}
	if p.DepartmentID != nil {
		afterSnap["department_id"] = after.DepartmentID
	}
	return s.audit.Record(ctx, audit.Entry{
		ActorID: actorID(ctx), Action: action, TargetType: "user", TargetID: id.String(),
		Result: audit.ResultSuccess, Before: beforeSnap, After: afterSnap,
	})
}

// guardLastSystemAdminLocked refuses a change that would leave zero active
// holders of RoleSystemAdmin. It is called only when the target currently
// holds that role and the patch would take it away or disable the account,
// so the count it checks includes the target — "1" means "just them."
//
// It must be called from inside the write's own transaction. A count taken
// before the transaction opens — this project's original implementation —
// is not sufficient: two concurrent requests, each demoting one of the last
// two system admins, can both read "2 remaining" before either writes, and
// both then proceed, leaving zero. LockActiveByRole's row lock is what
// forces the second request to see the first's committed result before it
// gets to decide: PostgreSQL re-evaluates a FOR UPDATE query's WHERE clause
// against each row's latest version once its lock is granted, so the second
// caller's count reflects the first caller's write, not the stale state its
// own transaction started with.
func (s *Service) guardLastSystemAdminLocked(ctx context.Context) error {
	n, err := s.repo.LockActiveByRole(ctx, auth.RoleSystemAdmin)
	if err != nil {
		return err
	}
	if n <= 1 {
		return apperr.Conflict("LAST_SYSTEM_ADMIN",
			"this would leave the organization with no active system administrator")
	}
	return nil
}

// ChangePassword sets a new password.
//
// It is separate from Update because it has a different authorization rule and
// a different input: the plaintext must never travel through the generic patch
// path, where it could end up logged alongside the other fields.
func (s *Service) ChangePassword(ctx context.Context, id ID, newPassword string) error {
	if err := auth.AuthorizeSelfOrPermission(ctx, id.String(), auth.PermUserUpdate); err != nil {
		return err
	}
	// A caller acting on someone else's password — not their own — must not
	// be able to reach a system administrator's account through the
	// ordinary user:update permission member_admin holds. See
	// auth.GuardActionOnAccount's doc comment.
	if principal, ok := auth.PrincipalFrom(ctx); ok && principal.UserID != id.String() {
		target, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if err := auth.GuardActionOnAccount(ctx, id.String(), target.Role); err != nil {
			return err
		}
	}
	if err := auth.ValidatePassword(newPassword); err != nil {
		return err
	}

	hash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return apperr.Internal(err)
	}
	return s.tx.WithTx(ctx, func(ctx context.Context) error {
		if _, err := s.repo.Update(ctx, id, Patch{PasswordHash: &hash}); err != nil {
			return err
		}
		// Before/After are deliberately empty: this exists to prove a
		// password was changed and by whom, never to carry the value or the
		// hash into the audit log (work requirement: no secret content in
		// audit records, ever).
		return s.audit.Record(ctx, audit.Entry{
			ActorID: actorID(ctx), Action: "member.password_changed",
			TargetType: "user", TargetID: id.String(), Result: audit.ResultSuccess,
		})
	})
}

// Delete removes a user. Unlike read and update, this is not available through
// ownership: self-deletion is a product decision that has not been made, and
// defaulting to allowing it would be irreversible.
func (s *Service) Delete(ctx context.Context, id ID) error {
	if err := auth.Authorize(ctx, auth.PermUserDelete); err != nil {
		return err
	}
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := auth.GuardActionOnAccount(ctx, id.String(), before.Role); err != nil {
		return err
	}
	return s.tx.WithTx(ctx, func(ctx context.Context) error {
		// Inside the transaction, not before it — see
		// guardLastSystemAdminLocked's doc comment.
		if before.Role == auth.RoleSystemAdmin {
			if err := s.guardLastSystemAdminLocked(ctx); err != nil {
				return err
			}
		}
		if err := s.repo.Delete(ctx, id); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{
			ActorID: actorID(ctx), Action: "member.deleted", TargetType: "user", TargetID: id.String(),
			Result: audit.ResultSuccess,
			Before: map[string]any{"email": before.Email, "role": before.Role.String()},
		})
	})
}

func actorID(ctx context.Context) *string {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil
	}
	id := p.UserID
	return &id
}
