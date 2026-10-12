package messaging

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// DeadLetter is a message that exhausted its retries and is parked for an
// operator to triage.
type DeadLetter struct {
	ID         uuid.UUID
	Event      Event
	Consumer   string
	Attempts   int32
	LastError  string
	FailedAt   time.Time
	ReplayedAt *time.Time
}

// OutboxStatus summarises the outbox for an operator: is the relay keeping
// up, and is anything stuck. It is the same four numbers `make
// outbox-status` prints.
type OutboxStatus struct {
	Pending       int64
	Published     int64
	Failing       int64
	WorstAttempts int32
}

// DeadLetterPage is a bounded slice of the dead-letter queue together with
// how many rows are outstanding in total.
//
// Total exists because Items is capped at a limit (see
// DefaultDeadLetterLimit) — without it, a client has no way to tell "this
// is everything" from "this is the first 50 of many", and a dashboard that
// counted len(Items) as the total would quietly under-report a queue larger
// than the page it happened to ask for.
type DeadLetterPage struct {
	Items []DeadLetter
	Total int64
}

// AdminStore is the admin API's view of the messaging tables. Declared here,
// next to its only consumer, the same way OutboxStore and DedupeStore are:
// the database package happens to satisfy this rather than exporting it, so
// the transport layer can depend on an interface that lives beside the
// domain types instead of on messagingpg directly.
type AdminStore interface {
	OutboxStatus(ctx context.Context) (OutboxStatus, error)
	ListDeadLetters(ctx context.Context, eventType string, limit int32) (DeadLetterPage, error)
	Replay(ctx context.Context, id uuid.UUID) error
}

// Default and maximum page sizes for the dead-letter listing. Kept small: this
// is a triage queue an operator reads end to end, not a paginated resource.
const (
	DefaultDeadLetterLimit = 50
	MaxDeadLetterLimit     = 200
)

// Admin exposes outbox and dead-letter operations to the HTTP layer.
//
// It adds essentially no rules of its own: whether a replay is possible is a
// database constraint (the row exists, it has not already been replayed),
// not a business one, and the store already enforces it. What this type
// buys is the same thing Service buys for users — the transport layer
// depends on messaging.AdminStore and messaging.DeadLetter, never on
// messagingpg or pgx.
type Admin struct {
	store AdminStore
}

// NewAdmin builds an Admin service.
func NewAdmin(store AdminStore) *Admin {
	return &Admin{store: store}
}

// OutboxStatus reports the current depth and failure counts.
func (a *Admin) OutboxStatus(ctx context.Context) (OutboxStatus, error) {
	return a.store.OutboxStatus(ctx)
}

// ListDeadLetters returns outstanding dead letters, newest first, optionally
// filtered to one event type. A non-positive or over-sized limit is clamped
// rather than rejected, matching how user listing treats page_size: asking
// for too many rows is a reasonable request, serving them all is not.
func (a *Admin) ListDeadLetters(ctx context.Context, eventType string, limit int32) (DeadLetterPage, error) {
	switch {
	case limit <= 0:
		limit = DefaultDeadLetterLimit
	case limit > MaxDeadLetterLimit:
		limit = MaxDeadLetterLimit
	}
	return a.store.ListDeadLetters(ctx, eventType, limit)
}

// ReplayDeadLetter puts a parked message back into the outbox.
func (a *Admin) ReplayDeadLetter(ctx context.Context, id uuid.UUID) error {
	return a.store.Replay(ctx, id)
}
