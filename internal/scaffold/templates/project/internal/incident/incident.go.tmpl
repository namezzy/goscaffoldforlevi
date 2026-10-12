// Package incident is the exception/processing center: it turns dead
// letters (package messaging) into triageable incidents with a severity, an
// owner, acknowledgement, and a real (not assumed) replay result. It knows
// nothing about HTTP or SQL — those live in incidenthttp and incidentpg.
//
// This is deliberately not a second messaging system. An incident stores a
// reference to the dead letter behind it (DeadLetterID) plus only the
// context needed to render a list (EventType, Consumer, LastError) — the
// full payload is read through the existing dead-letter endpoints, and
// replay is the existing messaging.Admin.ReplayDeadLetter, never
// reimplemented here.
//
// There is no separate incident timeline table, for the same reason ticket
// has none: an incident's processing record is audit_events rows with
// target_type = "incident", read back through ActivityReader.
package incident

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/validate"
)

// Severity is the operator-assigned triage level. Detection never guesses
// one: every new incident starts at SeverityWarning, and re-triage is an
// explicit action (Service.SetSeverity) — see docs/security.md for why
// automatic classification is out of scope this phase.
type Severity string

// The closed set of severities.
const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Valid reports whether s is one of the defined severities.
func (s Severity) Valid() bool {
	switch s {
	case SeverityInfo, SeverityWarning, SeverityCritical:
		return true
	}
	return false
}

// Status is the incident's place in its triage lifecycle.
type Status string

// The closed set of statuses.
const (
	StatusOpen         Status = "open"
	StatusAcknowledged Status = "acknowledged"
	StatusResolved     Status = "resolved"
)

// ReplayStatus tracks the real outcome of the last replay this incident
// requested. Nil means no replay has been requested yet. "Submitted for
// replay" (ReplayPending) is never presented as "processed successfully"
// (ReplaySucceeded) — see Watcher.ResolvePendingReplays for how the real
// result is learned.
type ReplayStatus string

// The closed set of replay outcomes.
const (
	ReplayPending   ReplayStatus = "pending"
	ReplaySucceeded ReplayStatus = "succeeded"
	ReplayFailed    ReplayStatus = "failed"
)

// Incident is the domain entity: one kind of recurring failure, merged from
// however many dead letters share its fingerprint.
type Incident struct {
	ID          string
	Fingerprint string
	Severity    Severity
	Status      Status
	OwnerID     *string
	// DeadLetterID and EventID identify the most recent occurrence — the
	// dead letter that would be replayed and the event the replay-result
	// check watches for in the dedupe log. They move together on every
	// merge (see MergeIncidentOccurrence's doc comment).
	DeadLetterID      string
	EventID           string
	EventType         string
	Consumer          string
	LastError         string
	OccurrenceCount   int32
	FirstSeenAt       time.Time
	LastSeenAt        time.Time
	AcknowledgedAt    *time.Time
	AcknowledgedBy    *string
	ResolvedAt        *time.Time
	ResolvedBy        *string
	ReplayStatus      *ReplayStatus
	ReplayRequestedAt *time.Time
	ReplayResultAt    *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// NewID mints a UUIDv7 for a new incident, matching ticket.NewID (ADR-0006).
func NewID() string { return uuid.Must(uuid.NewV7()).String() }

// Fingerprint identifies "this kind of failure" — the merge key. Exported
// so the watcher and any future caller compute it identically.
func Fingerprint(consumer, eventType string) string { return consumer + ":" + eventType }

// ListFilter narrows ListFiltered. A nil/empty field means "no constraint on
// this dimension."
type ListFilter struct {
	Status   *Status
	Severity *Severity
	OwnerID  *string
	Search   string
}

// Page is one slice of a listing together with the total a client needs to
// render pagination controls.
type Page struct {
	Items      []Incident
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

// Repository is Service's persistence port: reading and acting on incidents
// that already exist. Creating and merging them is Watcher's job, through
// the separate PollRepository below — Service never opens or merges an
// incident itself, only triages one the watcher already found.
type Repository interface {
	ListFiltered(ctx context.Context, f ListFilter, limit, offset int32) ([]Incident, error)
	CountFiltered(ctx context.Context, f ListFilter) (int64, error)
	// CountOpenBySeverity backs the exception center's summary strip: how
	// many non-resolved incidents at each severity.
	CountOpenBySeverity(ctx context.Context) (map[Severity]int64, error)
	Get(ctx context.Context, id string) (Incident, error)
	// GetForUpdate is Get with the row locked until the caller's transaction
	// ends, so a status check and the action it guards cannot be split by a
	// concurrent Resolve. Must run inside a transaction.
	GetForUpdate(ctx context.Context, id string) (Incident, error)
	// SetSeverity, AssignOwner, Acknowledge, Resolve and RequestReplay each
	// run their update under a row lock (FOR UPDATE on the read that
	// precedes them), so an operator action and the watcher's merge cannot
	// lose each other's write. There is no optimistic-concurrency version
	// column here, unlike Ticket: incidents are lower-stakes triage
	// metadata a background process also writes to, and a brief lock wait
	// is a better fit than surfacing a conflict to the operator over a
	// counter bump they were not looking at.
	SetSeverity(ctx context.Context, id string, severity Severity) (Incident, error)
	AssignOwner(ctx context.Context, id string, ownerID *string) (Incident, error)
	// Acknowledge and Resolve report false (with no error) when the
	// incident was not in the status the transition requires — see
	// Service.Acknowledge and Service.Resolve.
	Acknowledge(ctx context.Context, id, actorID string) (Incident, bool, error)
	Resolve(ctx context.Context, id, actorID string) (Incident, bool, error)
	RequestReplay(ctx context.Context, id string) (Incident, error)
}

// TxManager runs a function inside a database transaction, so an incident
// write and its audit entry commit atomically (ADR-0005).
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// AuditRecorder records a management action — matching ticket.AuditRecorder.
type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry) error
}

// ActivityReader reads back audit records for a target — declared here
// rather than depending on audit.Service, whose List/Get enforce
// audit:read, a permission an incident owner does not necessarily hold.
// Authorization for GetActivity is this package's own (incident:read),
// checked before this is ever called.
type ActivityReader interface {
	List(ctx context.Context, f audit.Filter, limit, offset int32) ([]audit.Record, error)
}

// ReplayTrigger submits a dead letter for replay — messaging.Admin, reached
// through this narrow interface instead of a direct dependency so this
// package never needs to know about messaging.AdminStore or messagingpg.
type ReplayTrigger interface {
	ReplayDeadLetter(ctx context.Context, id uuid.UUID) error
}

// Service holds the exception center's operator-facing use cases: list,
// inspect, triage. Detection and merging are Watcher's job (a separate
// background type, the same relationship messaging.Relay has to
// messaging.Admin) — Service only ever acts on an incident that already
// exists.
type Service struct {
	repo     Repository
	tx       TxManager
	audit    AuditRecorder
	activity ActivityReader
	replay   ReplayTrigger
}

// NewService wires the service.
func NewService(repo Repository, tx TxManager, auditRecorder AuditRecorder, activity ActivityReader, replay ReplayTrigger) *Service {
	return &Service{repo: repo, tx: tx, audit: auditRecorder, activity: activity, replay: replay}
}

// List returns a filtered, paginated page of incidents. Requires
// incident:read.
func (s *Service) List(ctx context.Context, f ListFilter, page, pageSize int) (Page, error) {
	if err := auth.Authorize(ctx, auth.PermIncidentRead); err != nil {
		return Page{}, err
	}
	if f.Status != nil && *f.Status != StatusOpen && *f.Status != StatusAcknowledged && *f.Status != StatusResolved {
		return Page{}, apperr.Invalid("INVALID_QUERY_PARAM", "status must be a defined incident status",
			apperr.Detail{Field: "status", Reason: "not a defined status"})
	}
	if f.Severity != nil && !f.Severity.Valid() {
		return Page{}, apperr.Invalid("INVALID_QUERY_PARAM", "severity must be a defined severity",
			apperr.Detail{Field: "severity", Reason: "not a defined severity"})
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
		return Page{Items: []Incident{}, Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}, nil
	}
	offset := int32((page - 1) * pageSize) //nolint:gosec // bounded above
	items, err := s.repo.ListFiltered(ctx, f, int32(pageSize), offset)
	if err != nil {
		return Page{}, err
	}
	return Page{Items: items, Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}, nil
}

// Summary reports the number of non-resolved incidents at each severity.
// Requires incident:read.
func (s *Service) Summary(ctx context.Context) (map[Severity]int64, error) {
	if err := auth.Authorize(ctx, auth.PermIncidentRead); err != nil {
		return nil, err
	}
	return s.repo.CountOpenBySeverity(ctx)
}

// Get returns one incident. Requires incident:read.
func (s *Service) Get(ctx context.Context, id string) (Incident, error) {
	if err := auth.Authorize(ctx, auth.PermIncidentRead); err != nil {
		return Incident{}, err
	}
	return s.repo.Get(ctx, id)
}

// GetActivity returns the incident's processing record, most recent first —
// audit_events rows for it. Requires incident:read.
func (s *Service) GetActivity(ctx context.Context, id string) ([]audit.Record, error) {
	if err := auth.Authorize(ctx, auth.PermIncidentRead); err != nil {
		return nil, err
	}
	if _, err := s.repo.Get(ctx, id); err != nil {
		return nil, err
	}
	targetType := "incident"
	return s.activity.List(ctx, audit.Filter{TargetType: &targetType, TargetID: &id}, 200, 0)
}

// lockUnresolved row-locks the incident and refuses one that is resolved.
// Resolved is terminal: re-triaging, annotating or replaying a closed
// incident would act on a problem the team has already signed off. Must run
// inside the action's transaction.
func (s *Service) lockUnresolved(ctx context.Context, id string) (Incident, error) {
	current, err := s.repo.GetForUpdate(ctx, id)
	if err != nil {
		return Incident{}, err
	}
	if current.Status == StatusResolved {
		return Incident{}, apperr.Conflict("INCIDENT_INVALID_TRANSITION", "incident is resolved")
	}
	return current, nil
}

func (s *Service) record(ctx context.Context, actorID, action, incidentID string, after any) error {
	return s.audit.Record(ctx, audit.Entry{
		ActorID: &actorID, Action: action, TargetType: "incident", TargetID: incidentID,
		Result: audit.ResultSuccess, After: after,
	})
}

// SetSeverity re-triages an incident. Requires incident:manage.
func (s *Service) SetSeverity(ctx context.Context, id string, severity Severity) (Incident, error) {
	if err := auth.Authorize(ctx, auth.PermIncidentManage); err != nil {
		return Incident{}, err
	}
	if !severity.Valid() {
		return Incident{}, apperr.Invalid("INVALID_ARGUMENT", "severity must be one of: info, warning, critical",
			apperr.Detail{Field: "severity", Reason: "not a defined severity"})
	}
	principal, _ := auth.PrincipalFrom(ctx)
	var updated Incident
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		if _, err := s.lockUnresolved(ctx, id); err != nil {
			return err
		}
		var err error
		updated, err = s.repo.SetSeverity(ctx, id, severity)
		if err != nil {
			return err
		}
		return s.record(ctx, principal.UserID, "incident.severity_set", id, map[string]any{"severity": severity})
	})
	if err != nil {
		return Incident{}, err
	}
	return updated, nil
}

// AssignOwner sets (or, with an empty ownerID, clears) who is handling this
// incident. Requires incident:manage.
func (s *Service) AssignOwner(ctx context.Context, id, ownerID string) (Incident, error) {
	if err := auth.Authorize(ctx, auth.PermIncidentManage); err != nil {
		return Incident{}, err
	}
	principal, _ := auth.PrincipalFrom(ctx)
	var ptr *string
	if ownerID != "" {
		ptr = &ownerID
	}
	var updated Incident
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		if _, err := s.lockUnresolved(ctx, id); err != nil {
			return err
		}
		var err error
		updated, err = s.repo.AssignOwner(ctx, id, ptr)
		if err != nil {
			return err
		}
		return s.record(ctx, principal.UserID, "incident.owner_assigned", id, map[string]any{"owner_id": ptr})
	})
	if err != nil {
		return Incident{}, err
	}
	return updated, nil
}

// Acknowledge marks an open incident seen, without resolving it. Requires
// incident:manage.
func (s *Service) Acknowledge(ctx context.Context, id string) (Incident, error) {
	if err := auth.Authorize(ctx, auth.PermIncidentManage); err != nil {
		return Incident{}, err
	}
	principal, _ := auth.PrincipalFrom(ctx)
	var updated Incident
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		var (
			ok  bool
			err error
		)
		updated, ok, err = s.repo.Acknowledge(ctx, id, principal.UserID)
		if err != nil {
			return err
		}
		if !ok {
			return apperr.Conflict("INCIDENT_INVALID_TRANSITION", "incident is not open")
		}
		return s.record(ctx, principal.UserID, "incident.acknowledged", id, nil)
	})
	if err != nil {
		return Incident{}, err
	}
	return updated, nil
}

// ProcessNote records a processing note on the incident, without changing
// its status — the exception-center equivalent of ticket.ProcessNote.
// Requires incident:manage.
func (s *Service) ProcessNote(ctx context.Context, id, note string) (Incident, error) {
	if err := auth.Authorize(ctx, auth.PermIncidentManage); err != nil {
		return Incident{}, err
	}
	var v validate.Violations
	v.Required("note", note)
	v.RuneLen("note", note, 1, 4000)
	v.NoControlChars("note", note)
	if v.Any() {
		return Incident{}, v.Err("INVALID_ARGUMENT", "invalid note")
	}
	principal, _ := auth.PrincipalFrom(ctx)
	var current Incident
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		if current, err = s.lockUnresolved(ctx, id); err != nil {
			return err
		}
		return s.record(ctx, principal.UserID, "incident.note_added", id, map[string]any{"note": note})
	})
	if err != nil {
		return Incident{}, err
	}
	return current, nil
}

// Resolve closes the incident's triage. A later, unrelated recurrence of the
// same fingerprint opens a fresh incident rather than reopening this one —
// see the exception_center migration. Requires incident:manage.
func (s *Service) Resolve(ctx context.Context, id string) (Incident, error) {
	if err := auth.Authorize(ctx, auth.PermIncidentManage); err != nil {
		return Incident{}, err
	}
	principal, _ := auth.PrincipalFrom(ctx)
	var updated Incident
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		var (
			ok  bool
			err error
		)
		updated, ok, err = s.repo.Resolve(ctx, id, principal.UserID)
		if err != nil {
			return err
		}
		if !ok {
			return apperr.Conflict("INCIDENT_INVALID_TRANSITION", "incident is already resolved")
		}
		return s.record(ctx, principal.UserID, "incident.resolved", id, nil)
	})
	if err != nil {
		return Incident{}, err
	}
	return updated, nil
}

// RequestReplay submits the incident's underlying dead letter for replay —
// reusing messaging.Admin.ReplayDeadLetter, never a second replay path — and
// marks the incident's replay outcome pending. It is pending, not
// successful: Watcher.ResolvePendingReplays is what later learns the real
// result. Requires incident:manage.
func (s *Service) RequestReplay(ctx context.Context, id string) (Incident, error) {
	if err := auth.Authorize(ctx, auth.PermIncidentManage); err != nil {
		return Incident{}, err
	}
	principal, _ := auth.PrincipalFrom(ctx)
	var updated Incident
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		current, err := s.lockUnresolved(ctx, id)
		if err != nil {
			return err
		}
		deadLetterID, err := uuid.Parse(current.DeadLetterID)
		if err != nil {
			return apperr.Internal(err)
		}
		if err := s.replay.ReplayDeadLetter(ctx, deadLetterID); err != nil {
			return err
		}
		updated, err = s.repo.RequestReplay(ctx, id)
		if err != nil {
			return err
		}
		return s.record(ctx, principal.UserID, "incident.replay_requested", id,
			map[string]any{"dead_letter_id": current.DeadLetterID})
	})
	if err != nil {
		return Incident{}, err
	}
	return updated, nil
}
