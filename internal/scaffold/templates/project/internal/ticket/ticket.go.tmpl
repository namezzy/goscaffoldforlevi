// Package ticket holds the ticket domain: a reusable "service request"
// business — submit, assign, process, complete, close — meant to
// demonstrate the workbench pattern (discover work, act on it, see the
// result) rather than to be a general workflow engine. It knows nothing
// about HTTP or SQL — those live in tickethttp and ticketpg.
//
// There is no separate activity/timeline table. A ticket's timeline is
// audit_events rows with target_type = "ticket" — the same append-only log
// every other feature's history already lives in (Phase 1) — read back
// through ActivityReader below, not through audit.Service.List: that method
// requires audit:read, a system-wide permission no ordinary requester or
// assignee holds, and a ticket's own participants must still be able to see
// its own history. ActivityReader is the audit *storage*, reused directly,
// authorized by this package's own ticket-level rules instead.
package ticket

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/notification"
	"github.com/namezzy/goscaffoldforlevi/internal/validate"
)

// Priority is the closed set of ticket priorities.
type Priority string

// The closed set of ticket priorities.
const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// Valid reports whether p is one of the defined priorities.
func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent:
		return true
	}
	return false
}

// Status is the ticket's place in its state machine. See Service's action
// methods (Assign, Reassign, ProcessNote, ProvideInfo, Complete, Close) for
// the transitions each one performs and who may perform them — that table
// exists once, there, rather than being duplicated here as a comment that
// could drift from the code that actually enforces it.
type Status string

// The closed set of ticket statuses.
const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusNeedsInfo  Status = "needs_info"
	StatusCompleted  Status = "completed"
	StatusClosed     Status = "closed"
)

// Ticket is the domain entity.
type Ticket struct {
	ID          string
	Title       string
	Description string
	Category    string
	Priority    Priority
	Status      Status
	RequesterID string
	AssigneeID  *string
	// Version is the optimistic-concurrency token: every mutating action
	// must supply the version it read, and the write increments it. A
	// stale version — someone else's action landed first — fails the
	// conditional UPDATE and the caller gets TICKET_VERSION_CONFLICT
	// instead of silently overwriting whatever that other action did.
	Version   int32
	CreatedAt time.Time
	UpdatedAt time.Time
	ClosedAt  *time.Time
}

// canAccess reports whether principal may read this ticket at all: its own
// requester, its current assignee, or a ticket:read_all holder. Every
// Service method that operates on an existing ticket starts from this.
func (t Ticket) canAccess(principal auth.Principal) bool {
	if principal.UserID == t.RequesterID {
		return true
	}
	if t.AssigneeID != nil && principal.UserID == *t.AssigneeID {
		return true
	}
	return principal.Role.Can(auth.PermTicketReadAll)
}

// NewID mints a UUIDv7 for a new ticket, matching user.NewID (ADR-0006).
func NewID() string { return uuid.Must(uuid.NewV7()).String() }

// ListFilter narrows ListFiltered. A nil/empty field means "no constraint
// on this dimension." RequesterID and AssigneeID are how the three list
// views (all / mine-as-requester / assigned-to-me) share one query — see
// Service.List.
type ListFilter struct {
	RequesterID *string
	AssigneeID  *string
	Status      *Status
	Priority    *Priority
	Search      string
	SortBy      SortBy
	SortDesc    bool
}

// SortBy is a column ListFiltered may order by.
type SortBy string

// The closed set of sortable columns.
const (
	SortByCreatedAt SortBy = "created_at"
	SortByPriority  SortBy = "priority"
	SortByStatus    SortBy = "status"
)

// Valid reports whether s is one of the defined sort columns.
func (s SortBy) Valid() bool {
	return s == SortByCreatedAt || s == SortByPriority || s == SortByStatus
}

// Repository is the persistence port.
type Repository interface {
	Create(ctx context.Context, t Ticket) (Ticket, error)
	GetByID(ctx context.Context, id string) (Ticket, error)
	ListFiltered(ctx context.Context, f ListFilter, limit, offset int32) ([]Ticket, error)
	CountFiltered(ctx context.Context, f ListFilter) (int64, error)
	// CountByStatus backs the workbench overview: one row per status
	// actually present.
	CountByStatus(ctx context.Context) (map[Status]int64, error)
	// Update applies the given change, conditioned on expectedVersion —
	// see Ticket.Version's doc comment. apperr.Conflict with code
	// TICKET_VERSION_CONFLICT when the row's current version does not
	// match.
	Update(ctx context.Context, id string, expectedVersion int32, p Patch) (Ticket, error)
}

// Patch describes a partial, conditional update — see Repository.Update.
// Every mutating Service method builds exactly one of these.
type Patch struct {
	Status        *Status
	AssigneeID    *string
	ClearAssignee bool
}

// TxManager runs a function inside a database transaction, so a ticket
// write, its audit entry and its notification all commit atomically
// (ADR-0005).
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// AuditRecorder records a management action — matching user.AuditRecorder's
// relationship to the audit package.
type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry) error
}

// ActivityReader reads back audit records for a target — declared here
// rather than depending on audit.Service, whose List/Get enforce
// audit:read, a permission a ticket's own requester or assignee does not
// hold and must not need in order to see their own ticket's history. The
// authorization for GetActivity is entirely this package's own (Ticket.
// canAccess); by the time this is called that has already been checked.
type ActivityReader interface {
	List(ctx context.Context, f audit.Filter, limit, offset int32) ([]audit.Record, error)
}

// NotificationRecorder creates an in-app notification — declared here
// rather than depending on notification.Service directly, the same
// relationship AuditRecorder has to package audit (ADR-0002).
type NotificationRecorder interface {
	Notify(ctx context.Context, n notification.NewNotification) error
}

// Service holds the ticket use cases.
//
// There is no EventPublisher here and tickets never reach the outbox:
// email delivery is explicitly optional per the product spec ("如接入邮件..."),
// and the in-app notification written by NotificationRecorder already
// satisfies every required delivery in the acceptance flow. Wiring an
// outbox event per action for a channel nothing consumes would be
// speculative.
type Service struct {
	repo     Repository
	tx       TxManager
	audit    AuditRecorder
	activity ActivityReader
	notify   NotificationRecorder
}

// NewService wires the service.
func NewService(
	repo Repository, tx TxManager, auditRecorder AuditRecorder, activity ActivityReader,
	notify NotificationRecorder,
) *Service {
	return &Service{repo: repo, tx: tx, audit: auditRecorder, activity: activity, notify: notify}
}

// NewTicketInput is the validated input for Create.
type NewTicketInput struct {
	Title       string
	Description string
	Category    string
	Priority    Priority
}

// Create submits a new ticket. Every role holds ticket:create — see that
// permission's doc comment — so submitting a request is available to
// anyone signed in, not just to roles that manage tickets.
func (s *Service) Create(ctx context.Context, in NewTicketInput) (Ticket, error) {
	if err := auth.Authorize(ctx, auth.PermTicketCreate); err != nil {
		return Ticket{}, err
	}
	principal, _ := auth.PrincipalFrom(ctx)

	var v validate.Violations
	v.Required("title", in.Title)
	v.RuneLen("title", in.Title, 1, 200)
	v.NoControlChars("title", in.Title)
	v.Required("description", in.Description)
	v.RuneLen("description", in.Description, 1, 8000)
	v.Required("category", in.Category)
	v.RuneLen("category", in.Category, 1, 100)
	if !in.Priority.Valid() {
		v.Add("priority", "must be one of: low, normal, high, urgent")
	}
	if v.Any() {
		return Ticket{}, v.Err("INVALID_ARGUMENT", "invalid ticket")
	}

	var created Ticket
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		created, err = s.repo.Create(ctx, Ticket{
			ID: NewID(), Title: in.Title, Description: in.Description, Category: in.Category,
			Priority: in.Priority, Status: StatusPending, RequesterID: principal.UserID,
		})
		if err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{
			ActorID: &principal.UserID, Action: "ticket.created", TargetType: "ticket", TargetID: created.ID,
			Result: audit.ResultSuccess,
			After:  map[string]any{"title": created.Title, "priority": created.Priority, "category": created.Category},
		})
	})
	if err != nil {
		return Ticket{}, err
	}
	return created, nil
}

// Get returns one ticket. Access is the requester, the current assignee, or
// a ticket:read_all holder — see Ticket.canAccess.
func (s *Service) Get(ctx context.Context, id string) (Ticket, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Ticket{}, err
	}
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return Ticket{}, auth.ErrMissingCredentials()
	}
	if !t.canAccess(principal) {
		return Ticket{}, auth.ErrForbidden()
	}
	return t, nil
}

// Page is one slice of a listing together with the total a client needs to
// render pagination controls.
type Page struct {
	Items      []Ticket
	Page       int
	PageSize   int
	Total      int64
	TotalPages int
}

// Pagination bounds, matching every other listing in this codebase
// (ADR-0004).
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
	maxOffset       = 1 << 30
)

// Scope narrows List to one of the three views the workbench offers.
type Scope string

const (
	// ScopeMine is tickets the caller submitted.
	ScopeMine Scope = "mine"
	// ScopeAssigned is tickets currently assigned to the caller.
	ScopeAssigned Scope = "assigned"
	// ScopeAll is every ticket — requires ticket:read_all.
	ScopeAll Scope = "all"
)

// List returns a filtered, paginated page of tickets, narrowed by scope.
// ScopeMine and ScopeAssigned need no permission beyond ticket:create
// (which everyone holds) — they are the caller's own tickets by
// construction, ownership rather than a grant. ScopeAll requires
// ticket:read_all.
func (s *Service) List(ctx context.Context, scope Scope, f ListFilter, page, pageSize int) (Page, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return Page{}, auth.ErrMissingCredentials()
	}
	switch scope {
	case ScopeMine:
		f.RequesterID = &principal.UserID
		f.AssigneeID = nil
	case ScopeAssigned:
		f.AssigneeID = &principal.UserID
		f.RequesterID = nil
	case ScopeAll:
		if err := auth.Authorize(ctx, auth.PermTicketReadAll); err != nil {
			return Page{}, err
		}
	default:
		return Page{}, apperr.Invalid("INVALID_ARGUMENT", "scope must be one of: mine, assigned, all",
			apperr.Detail{Field: "scope", Reason: "not a defined scope"})
	}

	if f.Status != nil && *f.Status != StatusPending && *f.Status != StatusInProgress &&
		*f.Status != StatusNeedsInfo && *f.Status != StatusCompleted && *f.Status != StatusClosed {
		return Page{}, apperr.Invalid("INVALID_QUERY_PARAM", "status must be a defined ticket status",
			apperr.Detail{Field: "status", Reason: "not a defined status"})
	}
	if f.Priority != nil && !f.Priority.Valid() {
		return Page{}, apperr.Invalid("INVALID_QUERY_PARAM", "priority must be a defined priority",
			apperr.Detail{Field: "priority", Reason: "not a defined priority"})
	}
	if f.SortBy == "" {
		f.SortBy = SortByCreatedAt
	}
	if !f.SortBy.Valid() {
		return Page{}, apperr.Invalid("INVALID_QUERY_PARAM", "sort must be one of: created_at, priority, status",
			apperr.Detail{Field: "sort", Reason: "not a sortable column"})
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
		return Page{Items: []Ticket{}, Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}, nil
	}
	offset := int32((page - 1) * pageSize) //nolint:gosec // bounded above
	items, err := s.repo.ListFiltered(ctx, f, int32(pageSize), offset)
	if err != nil {
		return Page{}, err
	}
	return Page{Items: items, Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}, nil
}

// StatusOverview reports how many tickets are in each status — the
// workbench's admin-facing ticket overview. Requires ticket:read_all: an
// org-wide count is exactly the thing that permission gates.
func (s *Service) StatusOverview(ctx context.Context) (map[Status]int64, error) {
	if err := auth.Authorize(ctx, auth.PermTicketReadAll); err != nil {
		return nil, err
	}
	return s.repo.CountByStatus(ctx)
}

// OpenStatuses is the set of statuses CountOpen treats as "still open":
// submitted but not yet completed or closed. It is the fixed definition
// behind the workbench's "my open tickets" / "assigned to me" counts —
// naming it here, once, is what keeps that definition from drifting
// between the count and whatever the workbench displays alongside it.
var OpenStatuses = []Status{StatusPending, StatusInProgress, StatusNeedsInfo}

// CountOpen reports how many of the caller's own tickets (scope mine or
// assigned; ScopeAll is refused — there is no organization-wide "open
// count" use case, only StatusOverview's full breakdown) are currently
// open. This is a current-state snapshot, not a windowed metric: there is
// no time range to misstate, only "as of right now."
func (s *Service) CountOpen(ctx context.Context, scope Scope) (int64, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return 0, auth.ErrMissingCredentials()
	}
	f := ListFilter{}
	switch scope {
	case ScopeMine:
		f.RequesterID = &principal.UserID
	case ScopeAssigned:
		f.AssigneeID = &principal.UserID
	default:
		return 0, apperr.Invalid("INVALID_ARGUMENT", "scope must be one of: mine, assigned",
			apperr.Detail{Field: "scope", Reason: "not a defined scope for CountOpen"})
	}
	var total int64
	for _, status := range OpenStatuses {
		status := status
		f.Status = &status
		n, err := s.repo.CountFiltered(ctx, f)
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

// GetActivity returns the ticket's timeline, most recent action first
// (ListAuditEventsFiltered's fixed order) — audit_events rows for it, in
// the same shape audit.Service.List returns them. Access is the same as
// Get's: the ticket's own requester or assignee, or a ticket:read_all
// holder, never a bare audit:read check.
func (s *Service) GetActivity(ctx context.Context, ticketID string) ([]audit.Record, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, auth.ErrMissingCredentials()
	}
	t, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if !t.canAccess(principal) {
		return nil, auth.ErrForbidden()
	}
	targetType := "ticket"
	records, err := s.activity.List(ctx, audit.Filter{TargetType: &targetType, TargetID: &ticketID}, 200, 0)
	if err != nil {
		return nil, err
	}
	return records, nil
}

func dedupKey(ticketID string, event string, version int32) string {
	return fmt.Sprintf("ticket:%s:%s:v%d", ticketID, event, version)
}

func requireStatus(t Ticket, allowed ...Status) error {
	for _, want := range allowed {
		if t.Status == want {
			return nil
		}
	}
	return apperr.Conflict("TICKET_INVALID_TRANSITION",
		fmt.Sprintf("ticket is %s and this action is not allowed from that status", t.Status))
}

// transitionInput is one state-machine action: fetch, authorize, update
// under optimistic concurrency, record an audit entry, optionally notify.
// Every action method below (Assign, Reassign, ProcessNote, RequestInfo,
// ProvideInfo, Complete, Close) builds one of these and calls transition —
// the fetch-authorize-update-audit-notify sequence is identical across all
// seven, only what each step checks and writes differs.
type transitionInput struct {
	TicketID        string
	ExpectedVersion int32
	Note            string
	Patch           Patch
	Action          string
	// Authorize runs against the ticket as it was read, before the update.
	// It reports both permission and current-status eligibility, since for
	// this state machine the two are checked together everywhere they are
	// used.
	Authorize func(t Ticket) error
	// Notify, when non-nil, is called with the pre- and post-update ticket
	// to build the in-app notification for this action, or nil to send
	// none. It runs inside the same transaction as the update and audit
	// entry, so a failure to notify rolls the whole action back rather
	// than leaving the ticket changed with nobody told.
	Notify func(before, after Ticket) *notification.NewNotification
}

func (s *Service) transition(ctx context.Context, in transitionInput) (Ticket, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return Ticket{}, auth.ErrMissingCredentials()
	}
	before, err := s.repo.GetByID(ctx, in.TicketID)
	if err != nil {
		return Ticket{}, err
	}
	if err := in.Authorize(before); err != nil {
		return Ticket{}, err
	}
	// Authorization above judged the ticket at before.Version. The write
	// must be conditioned on that same version, never on a client-supplied
	// one that could be pre-filled to a later version to carry this
	// snapshot's authorization past a concurrent change.
	if in.ExpectedVersion != before.Version {
		return Ticket{}, apperr.Conflict("TICKET_VERSION_CONFLICT",
			"the ticket changed since it was loaded; reload it and try again")
	}

	var after Ticket
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		after, err = s.repo.Update(ctx, in.TicketID, before.Version, in.Patch)
		if err != nil {
			return err
		}
		entry := audit.Entry{
			ActorID: &principal.UserID, Action: in.Action, TargetType: "ticket", TargetID: in.TicketID,
			Result: audit.ResultSuccess, Before: map[string]any{"status": before.Status},
			After: map[string]any{"status": after.Status},
		}
		if in.Note != "" {
			entry.After = map[string]any{"status": after.Status, "note": in.Note}
		}
		if err := s.audit.Record(ctx, entry); err != nil {
			return err
		}
		if in.Notify == nil {
			return nil
		}
		n := in.Notify(before, after)
		if n == nil {
			return nil
		}
		return s.notify.Notify(ctx, *n)
	})
	if err != nil {
		return Ticket{}, err
	}
	return after, nil
}

// Assign dispatches a pending ticket to assignee, moving it to in_progress.
// Only a ticket:manage holder may dispatch.
func (s *Service) Assign(ctx context.Context, ticketID, assigneeID string, expectedVersion int32) (Ticket, error) {
	var v validate.Violations
	v.Required("assignee_id", assigneeID)
	if v.Any() {
		return Ticket{}, v.Err("INVALID_ARGUMENT", "invalid assignment")
	}
	status := StatusInProgress
	return s.transition(ctx, transitionInput{
		TicketID: ticketID, ExpectedVersion: expectedVersion, Action: "ticket.assigned",
		Patch: Patch{Status: &status, AssigneeID: &assigneeID},
		Authorize: func(t Ticket) error {
			principal, _ := auth.PrincipalFrom(ctx)
			if !principal.Role.Can(auth.PermTicketManage) {
				return auth.ErrForbidden()
			}
			return requireStatus(t, StatusPending)
		},
		Notify: func(before, after Ticket) *notification.NewNotification {
			return &notification.NewNotification{
				UserID: assigneeID, Type: "ticket_assigned", Title: "New ticket assigned to you",
				Body: after.Title, TargetType: "ticket", TargetID: after.ID,
				DedupKey: dedupKey(after.ID, "assigned", after.Version),
			}
		},
	})
}

// Reassign moves an already-dispatched ticket to a different assignee
// without changing its status. Only a ticket:manage holder may reassign.
func (s *Service) Reassign(ctx context.Context, ticketID, newAssigneeID string, expectedVersion int32) (Ticket, error) {
	var v validate.Violations
	v.Required("assignee_id", newAssigneeID)
	if v.Any() {
		return Ticket{}, v.Err("INVALID_ARGUMENT", "invalid assignment")
	}
	return s.transition(ctx, transitionInput{
		TicketID: ticketID, ExpectedVersion: expectedVersion, Action: "ticket.reassigned",
		Patch: Patch{AssigneeID: &newAssigneeID},
		Authorize: func(t Ticket) error {
			principal, _ := auth.PrincipalFrom(ctx)
			if !principal.Role.Can(auth.PermTicketManage) {
				return auth.ErrForbidden()
			}
			return requireStatus(t, StatusInProgress, StatusNeedsInfo)
		},
		Notify: func(before, after Ticket) *notification.NewNotification {
			return &notification.NewNotification{
				UserID: newAssigneeID, Type: "ticket_assigned", Title: "Ticket reassigned to you",
				Body: after.Title, TargetType: "ticket", TargetID: after.ID,
				DedupKey: dedupKey(after.ID, "assigned", after.Version),
			}
		},
	})
}

// isCurrentAssignee reports whether principal is t's current assignee.
func isCurrentAssignee(t Ticket, principal auth.Principal) bool {
	return t.AssigneeID != nil && *t.AssigneeID == principal.UserID
}

// ProcessNote records a processing note on an in-progress or needs_info
// ticket without changing its status — available to the current assignee
// or a ticket:manage holder. It still goes through the optimistic-
// concurrency update (bumping Version with an empty Patch) so a stale page
// cannot silently sit on top of a note added by someone else in the
// meantime.
func (s *Service) ProcessNote(ctx context.Context, ticketID, note string, expectedVersion int32) (Ticket, error) {
	var v validate.Violations
	v.Required("note", note)
	v.RuneLen("note", note, 1, 4000)
	v.NoControlChars("note", note)
	if v.Any() {
		return Ticket{}, v.Err("INVALID_ARGUMENT", "invalid note")
	}
	return s.transition(ctx, transitionInput{
		TicketID: ticketID, ExpectedVersion: expectedVersion, Action: "ticket.note_added", Note: note,
		Authorize: func(t Ticket) error {
			principal, _ := auth.PrincipalFrom(ctx)
			if !isCurrentAssignee(t, principal) && !principal.Role.Can(auth.PermTicketManage) {
				return auth.ErrForbidden()
			}
			return requireStatus(t, StatusInProgress, StatusNeedsInfo)
		},
	})
}

// RequestInfo asks the requester for more information, moving the ticket
// to needs_info. Available to the current assignee or a ticket:manage
// holder, only from in_progress.
func (s *Service) RequestInfo(ctx context.Context, ticketID, note string, expectedVersion int32) (Ticket, error) {
	var v validate.Violations
	v.Required("note", note)
	v.RuneLen("note", note, 1, 4000)
	v.NoControlChars("note", note)
	if v.Any() {
		return Ticket{}, v.Err("INVALID_ARGUMENT", "invalid note")
	}
	status := StatusNeedsInfo
	return s.transition(ctx, transitionInput{
		TicketID: ticketID, ExpectedVersion: expectedVersion, Action: "ticket.info_requested", Note: note,
		Patch: Patch{Status: &status},
		Authorize: func(t Ticket) error {
			principal, _ := auth.PrincipalFrom(ctx)
			if !isCurrentAssignee(t, principal) && !principal.Role.Can(auth.PermTicketManage) {
				return auth.ErrForbidden()
			}
			return requireStatus(t, StatusInProgress)
		},
		Notify: func(before, after Ticket) *notification.NewNotification {
			return &notification.NewNotification{
				UserID: after.RequesterID, Type: "ticket_needs_info", Title: "More information needed on your ticket",
				Body: note, TargetType: "ticket", TargetID: after.ID,
				DedupKey: dedupKey(after.ID, "needs_info", after.Version),
			}
		},
	})
}

// ProvideInfo answers a needs_info request, moving the ticket back to
// in_progress. Only the requester may supply it.
func (s *Service) ProvideInfo(ctx context.Context, ticketID, note string, expectedVersion int32) (Ticket, error) {
	var v validate.Violations
	v.Required("note", note)
	v.RuneLen("note", note, 1, 4000)
	v.NoControlChars("note", note)
	if v.Any() {
		return Ticket{}, v.Err("INVALID_ARGUMENT", "invalid note")
	}
	status := StatusInProgress
	return s.transition(ctx, transitionInput{
		TicketID: ticketID, ExpectedVersion: expectedVersion, Action: "ticket.info_provided", Note: note,
		Patch: Patch{Status: &status},
		Authorize: func(t Ticket) error {
			principal, _ := auth.PrincipalFrom(ctx)
			if principal.UserID != t.RequesterID {
				return auth.ErrForbidden()
			}
			return requireStatus(t, StatusNeedsInfo)
		},
	})
}

// Complete marks an in-progress ticket done. Available to the current
// assignee or a ticket:manage holder.
func (s *Service) Complete(ctx context.Context, ticketID, note string, expectedVersion int32) (Ticket, error) {
	var v validate.Violations
	if note != "" {
		v.RuneLen("note", note, 1, 4000)
		v.NoControlChars("note", note)
	}
	if v.Any() {
		return Ticket{}, v.Err("INVALID_ARGUMENT", "invalid note")
	}
	status := StatusCompleted
	return s.transition(ctx, transitionInput{
		TicketID: ticketID, ExpectedVersion: expectedVersion, Action: "ticket.completed", Note: note,
		Patch: Patch{Status: &status},
		Authorize: func(t Ticket) error {
			principal, _ := auth.PrincipalFrom(ctx)
			if !isCurrentAssignee(t, principal) && !principal.Role.Can(auth.PermTicketManage) {
				return auth.ErrForbidden()
			}
			return requireStatus(t, StatusInProgress)
		},
		Notify: func(before, after Ticket) *notification.NewNotification {
			return &notification.NewNotification{
				UserID: after.RequesterID, Type: "ticket_completed", Title: "Your ticket has been completed",
				Body: after.Title, TargetType: "ticket", TargetID: after.ID,
				DedupKey: dedupKey(after.ID, "completed", after.Version),
			}
		},
	})
}

// Close ends a ticket's lifecycle. The requester may close it once it is
// completed, confirming the result; a ticket:manage holder may close it
// from any non-closed status, for example to cancel an invalid request.
func (s *Service) Close(ctx context.Context, ticketID string, expectedVersion int32) (Ticket, error) {
	status := StatusClosed
	return s.transition(ctx, transitionInput{
		TicketID: ticketID, ExpectedVersion: expectedVersion, Action: "ticket.closed",
		Patch: Patch{Status: &status},
		Authorize: func(t Ticket) error {
			principal, _ := auth.PrincipalFrom(ctx)
			if principal.Role.Can(auth.PermTicketManage) {
				return requireStatus(t, StatusPending, StatusInProgress, StatusNeedsInfo, StatusCompleted)
			}
			if principal.UserID != t.RequesterID {
				return auth.ErrForbidden()
			}
			return requireStatus(t, StatusCompleted)
		},
	})
}
