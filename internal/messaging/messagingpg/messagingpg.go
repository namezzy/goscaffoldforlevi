// Package messagingpg is the PostgreSQL side of messaging: the transactional
// outbox, the consumer dedupe log, and the dead letter table.
//
// It is the only messaging code that knows about pgx or generated queries.
// Everything above it works with messaging.Event.
package messagingpg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/go-enterprise/internal/messaging"
	"github.com/example/go-enterprise/internal/postgres"
	"github.com/example/go-enterprise/internal/postgres/db"
)

// OutboxPublisher writes events to the outbox table.
//
// It is a Publisher, so a service publishes through the same interface whether
// the event ends up in NATS, in this table, or nowhere. The difference that
// matters is invisible from above and is the entire reason the table exists:
// this Publish participates in the caller's transaction, so the event and the
// change it describes commit together or not at all.
//
// The failure it prevents is not exotic. Publishing after the commit loses the
// event whenever the process dies in between, and leaves no trace that it did;
// publishing before the commit announces changes that then roll back. Only a
// write in the same transaction has neither failure mode.
type OutboxPublisher struct {
	pool *pgxpool.Pool
}

// NewOutboxPublisher returns a publisher backed by the outbox table.
func NewOutboxPublisher(pool *pgxpool.Pool) *OutboxPublisher {
	return &OutboxPublisher{pool: pool}
}

// Publish records the event in the outbox.
//
// It uses the ambient transaction when there is one. Calling it outside a
// transaction is allowed and still durable — the event is simply committed on
// its own, which is right for an event that describes no database change.
func (p *OutboxPublisher) Publish(ctx context.Context, event messaging.Event) error {
	queries := db.New(postgres.Querier(ctx, p.pool))

	err := queries.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{
		ID:          event.ID,
		EventType:   event.Type,
		Payload:     event.Payload,
		Traceparent: nullableString(event.TraceParent),
	})
	if err != nil {
		return fmt.Errorf("insert outbox event %s: %w", event.Type, err)
	}
	return nil
}

// Store is the relay's and the consumer's view of the messaging tables.
type Store struct {
	pool *pgxpool.Pool
	txm  postgres.TxManager
}

// NewStore returns a store backed by pool.
func NewStore(pool *pgxpool.Pool, txm postgres.TxManager) *Store {
	return &Store{pool: pool, txm: txm}
}

// ClaimBatch locks up to limit undelivered events and hands them to fn.
//
// The lock lives for the duration of fn's transaction, which is what lets
// several relays run at once: SKIP LOCKED means a second relay takes different
// rows instead of waiting for the first. fn receives a context carrying that
// transaction, so marking a row published happens under the same lock that
// claimed it.
//
// Delivery therefore happens while a transaction is open. That is a deliberate
// trade: holding the lock across the publish is what makes "published but not
// marked" the only possible inconsistency, and that one is harmless because
// consumers deduplicate. The alternative — release, publish, re-lock — allows
// two relays to publish the same event concurrently.
func (s *Store) ClaimBatch(
	ctx context.Context,
	limit int32,
	fn func(ctx context.Context, events []messaging.PendingOutboxEvent) error,
) error {
	return s.txm.WithTx(ctx, func(ctx context.Context) error {
		queries := db.New(postgres.Querier(ctx, s.pool))

		rows, err := queries.ClaimOutboxEvents(ctx, limit)
		if err != nil {
			return fmt.Errorf("claim outbox events: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}

		pending := make([]messaging.PendingOutboxEvent, 0, len(rows))
		for _, row := range rows {
			pending = append(pending, messaging.PendingOutboxEvent{
				Event: messaging.Event{
					ID:          row.ID,
					Type:        row.EventType,
					OccurredAt:  row.CreatedAt,
					TraceParent: derefString(row.Traceparent),
					Payload:     row.Payload,
				},
				Attempts: row.Attempts,
			})
		}
		return fn(ctx, pending)
	})
}

// MarkPublished records a delivered event.
func (s *Store) MarkPublished(ctx context.Context, id uuid.UUID) error {
	queries := db.New(postgres.Querier(ctx, s.pool))
	if err := queries.MarkOutboxEventPublished(ctx, id); err != nil {
		return fmt.Errorf("mark outbox event published: %w", err)
	}
	return nil
}

// RecordFailure counts a failed delivery without giving up on the event.
//
// The row stays unpublished on purpose. A broker that will not accept a
// message is an outage, and an outage that silently discards events is how a
// system loses data it never knows it lost.
func (s *Store) RecordFailure(ctx context.Context, id uuid.UUID, cause error) error {
	queries := db.New(postgres.Querier(ctx, s.pool))
	err := queries.RecordOutboxEventFailure(ctx, db.RecordOutboxEventFailureParams{
		ID:        id,
		LastError: nullableString(truncateError(cause)),
	})
	if err != nil {
		return fmt.Errorf("record outbox failure: %w", err)
	}
	return nil
}

// PendingCount reports how many events are waiting. It answers the only
// question an operator asks about a relay: is it keeping up.
func (s *Store) PendingCount(ctx context.Context) (int64, error) {
	queries := db.New(postgres.Querier(ctx, s.pool))
	count, err := queries.CountUnpublishedOutboxEvents(ctx)
	if err != nil {
		return 0, fmt.Errorf("count unpublished outbox events: %w", err)
	}
	return count, nil
}

// PrunePublished deletes events delivered before cutoff.
func (s *Store) PrunePublished(ctx context.Context, cutoff time.Time) (int64, error) {
	queries := db.New(postgres.Querier(ctx, s.pool))
	deleted, err := queries.DeletePublishedOutboxEventsBefore(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune published outbox events: %w", err)
	}
	return deleted, nil
}

// PruneProcessed deletes dedupe records older than cutoff.
//
// The cutoff must stay longer than the longest redelivery window. Pruning too
// eagerly does not merely lose history: it makes an old message processable a
// second time, which is the exact failure the log exists to prevent.
func (s *Store) PruneProcessed(ctx context.Context, cutoff time.Time) (int64, error) {
	queries := db.New(postgres.Querier(ctx, s.pool))
	deleted, err := queries.DeleteProcessedEventsBefore(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune processed events: %w", err)
	}
	return deleted, nil
}

// WithTx runs fn in a transaction, so the consumer can put the dedupe claim
// and the side effect in the same commit.
func (s *Store) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return s.txm.WithTx(ctx, fn)
}

// ClaimEvent reports whether this consumer still has to process the event,
// recording the claim if so.
//
// It must be called inside the same transaction as the side effect. Split
// across two transactions, the process can die between them and either lose
// the work or suppress it forever, and both outcomes look like the message was
// handled.
func (s *Store) ClaimEvent(
	ctx context.Context, eventID uuid.UUID, consumer string,
) (bool, error) {
	if !postgres.InTx(ctx) {
		// Not a defensive nicety: outside a transaction this function silently
		// stops providing the guarantee its callers depend on, and the bug
		// only appears as a duplicated side effect much later.
		return false, errors.New(
			"messagingpg: ClaimEvent must run inside the transaction that " +
				"performs the side effect")
	}

	queries := db.New(postgres.Querier(ctx, s.pool))
	inserted, err := queries.ClaimEventForConsumer(ctx, db.ClaimEventForConsumerParams{
		EventID:  eventID,
		Consumer: consumer,
	})
	if err != nil {
		return false, fmt.Errorf("claim event for consumer: %w", err)
	}
	return inserted == 1, nil
}

// RecordDeadLetter parks an event that exhausted its retries.
func (s *Store) RecordDeadLetter(
	ctx context.Context, event messaging.Event, consumer string, attempts int, cause error,
) error {
	queries := db.New(postgres.Querier(ctx, s.pool))
	err := queries.RecordDeadLetter(ctx, db.RecordDeadLetterParams{
		ID:          uuid.Must(uuid.NewV7()),
		EventID:     event.ID,
		EventType:   event.Type,
		Payload:     event.Payload,
		Traceparent: nullableString(event.TraceParent),
		Consumer:    consumer,
		Attempts:    int32(attempts), //nolint:gosec // bounded by the retry cap
		LastError:   truncateError(cause),
	})
	if err != nil {
		return fmt.Errorf("record dead letter: %w", err)
	}
	return nil
}

// DeadLetter is a parked message.
type DeadLetter struct {
	ID         uuid.UUID
	Event      messaging.Event
	Consumer   string
	Attempts   int32
	LastError  string
	FailedAt   time.Time
	ReplayedAt *time.Time
}

// ListDeadLetters returns outstanding dead letters, newest first.
func (s *Store) ListDeadLetters(
	ctx context.Context, eventType string, limit int32,
) ([]DeadLetter, error) {
	queries := db.New(postgres.Querier(ctx, s.pool))
	rows, err := queries.ListDeadLetters(ctx, db.ListDeadLettersParams{
		Limit:     limit,
		EventType: nullableString(eventType),
	})
	if err != nil {
		return nil, fmt.Errorf("list dead letters: %w", err)
	}

	letters := make([]DeadLetter, 0, len(rows))
	for _, row := range rows {
		letters = append(letters, DeadLetter{
			ID: row.ID,
			Event: messaging.Event{
				ID:          row.EventID,
				Type:        row.EventType,
				OccurredAt:  row.FailedAt,
				TraceParent: derefString(row.Traceparent),
				Payload:     row.Payload,
			},
			Consumer:   row.Consumer,
			Attempts:   row.Attempts,
			LastError:  row.LastError,
			FailedAt:   row.FailedAt,
			ReplayedAt: row.ReplayedAt,
		})
	}
	return letters, nil
}

// Replay puts a dead letter back into the outbox.
//
// The event keeps its identity, which is what makes replay safe to run twice:
// any consumer that did manage to process it will recognise the ID and skip
// it. Replaying under a new ID would defeat every dedupe record in the system.
func (s *Store) Replay(ctx context.Context, id uuid.UUID) error {
	return s.txm.WithTx(ctx, func(ctx context.Context) error {
		queries := db.New(postgres.Querier(ctx, s.pool))

		row, err := queries.GetDeadLetter(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("dead letter %s not found", id)
			}
			return fmt.Errorf("get dead letter: %w", err)
		}

		marked, err := queries.MarkDeadLetterReplayed(ctx, id)
		if err != nil {
			return fmt.Errorf("mark dead letter replayed: %w", err)
		}
		if marked == 0 {
			return fmt.Errorf("dead letter %s was already replayed", id)
		}

		// A fresh outbox row, because the original was published long ago and
		// the relay only looks at unpublished rows.
		err = queries.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{
			ID:          row.EventID,
			EventType:   row.EventType,
			Payload:     row.Payload,
			Traceparent: row.Traceparent,
		})
		if err != nil {
			return fmt.Errorf("requeue dead letter %s: %w", id, err)
		}
		return nil
	})
}

// maxErrorLength bounds what is stored from an error. A driver can return a
// multi-kilobyte message, and a table of them is a table nobody reads.
const maxErrorLength = 1000

func truncateError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) > maxErrorLength {
		return msg[:maxErrorLength] + "… (truncated)"
	}
	return msg
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
