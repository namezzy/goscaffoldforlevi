// Package audit owns the append-only management-action log: who did what, to
// what, when, with what result, and — where it applies — what changed.
//
// It knows nothing about HTTP or SQL; those live in audithttp and auditpg.
// Unlike every other business feature in this codebase, this package has no
// write endpoint of its own. Record is called directly by the service whose
// action is being audited (user, org, invite), inside that service's own
// transaction, so that a successful write and its audit record are a single
// commit — see docs/adr/0005-data-access.md's TxManager section. The database
// backs this further: audit_events rejects UPDATE and DELETE outright (the
// org_roles_audit migration), so even a bug in this package cannot alter
// history, only fail to add to it.
package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
)

// Result is the outcome of the audited action. A denied or failed attempt is
// still worth a record — "someone tried and was refused" is exactly the kind
// of event an audit log exists to answer questions about later.
type Result string

// The two possible outcomes.
const (
	ResultSuccess Result = "success"
	ResultFailure Result = "failure"
)

// Entry is one action to record.
//
// ActorID is nil for a system-initiated event (an invitation discovered
// expired on access, say) rather than one a signed-in principal performed.
// Before/After are arbitrary JSON-marshalable snapshots of only the fields
// that changed — never a whole record, so that a field added to a struct
// later (a password hash, a token) cannot silently start flowing into the
// audit log just because a caller passed the struct as a convenience.
type Entry struct {
	ActorID    *string
	Action     string
	TargetType string
	TargetID   string
	Result     Result
	RequestID  string
	Before     any
	After      any
}

// Record is a persisted entry, as returned by List and Get.
type Record struct {
	ID         string
	ActorID    *string
	Action     string
	TargetType string
	TargetID   string
	Result     Result
	RequestID  string
	Before     json.RawMessage
	After      json.RawMessage
	CreatedAt  time.Time
}

// Filter narrows List/Count. A nil field means "no constraint on this
// dimension." TargetType/TargetID together are what the member detail page
// uses to show events related to one person rather than events they
// performed — see queries/audit.sql's ListAuditEventsFiltered.
type Filter struct {
	From, To             *time.Time
	ActorID              *string
	Action               *string
	Result               *Result
	TargetType, TargetID *string
}

// Store is the persistence port.
type Store interface {
	Insert(ctx context.Context, r Record) error
	List(ctx context.Context, f Filter, limit, offset int32) ([]Record, error)
	Count(ctx context.Context, f Filter) (int64, error)
	Get(ctx context.Context, id string) (Record, error)
}

// Pagination bounds, matching user.Service's (ADR-0004): a page cannot be
// asked to be a full table scan in disguise.
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// Service is the audit use cases: recording, and reading back.
type Service struct {
	store Store
}

// NewService wires the service.
func NewService(store Store) *Service { return &Service{store: store} }

// Record persists an entry.
//
// It has no permission check. Writing an audit record is not an action any
// principal is authorized to take directly — there is no "create an audit
// event" capability in the policy — it is always a side effect of another
// action that is itself authorized by its own service. Gating this method
// would only let a caller who already passed that check fail to leave a
// trace, which is the one outcome the whole feature exists to prevent.
func (s *Service) Record(ctx context.Context, e Entry) error {
	before, err := marshal(e.Before)
	if err != nil {
		return apperr.Internal(err)
	}
	after, err := marshal(e.After)
	if err != nil {
		return apperr.Internal(err)
	}
	if e.Result == "" {
		e.Result = ResultSuccess
	}
	return s.store.Insert(ctx, Record{
		ID:         uuid.Must(uuid.NewV7()).String(),
		ActorID:    e.ActorID,
		Action:     e.Action,
		TargetType: e.TargetType,
		TargetID:   e.TargetID,
		Result:     e.Result,
		RequestID:  e.RequestID,
		Before:     before,
		After:      after,
		CreatedAt:  time.Now(),
	})
}

func marshal(v any) (json.RawMessage, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Page is one slice of a listing together with the total a client needs to
// render pagination controls.
type Page struct {
	Items      []Record
	Page       int
	PageSize   int
	Total      int64
	TotalPages int
}

// List returns a filtered, paginated slice of the audit log.
func (s *Service) List(ctx context.Context, f Filter, page, pageSize int) (Page, error) {
	if err := auth.Authorize(ctx, auth.PermAuditRead); err != nil {
		return Page{}, err
	}
	page, pageSize = normalizePaging(page, pageSize)

	total, err := s.store.Count(ctx, f)
	if err != nil {
		return Page{}, err
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	offset := int32((page - 1) * pageSize)                      //nolint:gosec // bounded by normalizePaging
	items, err := s.store.List(ctx, f, int32(pageSize), offset) //nolint:gosec // <= MaxPageSize
	if err != nil {
		return Page{}, err
	}
	return Page{Items: items, Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}, nil
}

// Get returns one audit record by ID.
func (s *Service) Get(ctx context.Context, id string) (Record, error) {
	if err := auth.Authorize(ctx, auth.PermAuditRead); err != nil {
		return Record{}, err
	}
	return s.store.Get(ctx, id)
}

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
