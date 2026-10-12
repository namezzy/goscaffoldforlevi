// Package notificationpg is the PostgreSQL implementation of the
// notification repository.
package notificationpg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/notification"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/db"
)

const codeForeignKeyViolation = "23503"

// Repository stores notifications.
type Repository struct {
	pool *pgxpool.Pool
}

// New returns a Repository backed by pool.
func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) q(ctx context.Context) *db.Queries {
	return db.New(postgres.Querier(ctx, r.pool))
}

// Create inserts a notification, or does nothing if (user_id, dedup_key)
// already exists — see notification.Store's doc comment. It participates
// in the caller's transaction via postgres.Querier, the same as every
// other repository in this codebase, which is what lets ticket.Service
// write the ticket action and its notification as one commit.
func (r *Repository) Create(ctx context.Context, id string, n notification.NewNotification) error {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return apperr.Internal(fmt.Errorf("notification id: %w", err))
	}
	userID, err := uuid.Parse(n.UserID)
	if err != nil {
		return apperr.Internal(fmt.Errorf("notification user id: %w", err))
	}
	_, err = r.q(ctx).CreateNotification(ctx, db.CreateNotificationParams{
		ID: parsedID, UserID: userID, Type: n.Type, Title: n.Title, Body: n.Body,
		TargetType: n.TargetType, TargetID: n.TargetID, DedupKey: n.DedupKey,
	})
	if err != nil {
		return translate(err, "create notification")
	}
	return nil
}

// ListFiltered returns one page of userID's notifications, most recent
// first.
func (r *Repository) ListFiltered(ctx context.Context, userID string, unreadOnly bool, limit, offset int32) ([]notification.Notification, error) {
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("notification user id: %w", err))
	}
	rows, err := r.q(ctx).ListNotificationsFiltered(ctx, db.ListNotificationsFilteredParams{
		Limit: limit, Offset: offset, UserID: parsed, UnreadOnly: unreadOnly,
	})
	if err != nil {
		return nil, translate(err, "list notifications")
	}
	out := make([]notification.Notification, len(rows))
	for i, row := range rows {
		out[i] = notification.Notification{
			ID: row.ID.String(), UserID: row.UserID.String(), Type: row.Type,
			Title: row.Title, Body: row.Body, TargetType: row.TargetType, TargetID: row.TargetID,
			ReadAt: row.ReadAt, CreatedAt: row.CreatedAt,
		}
	}
	return out, nil
}

// CountFiltered returns the total number of userID's notifications matching
// the same filter ListFiltered uses, for pagination metadata.
func (r *Repository) CountFiltered(ctx context.Context, userID string, unreadOnly bool) (int64, error) {
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return 0, apperr.Internal(fmt.Errorf("notification user id: %w", err))
	}
	n, err := r.q(ctx).CountNotificationsFiltered(ctx, db.CountNotificationsFilteredParams{
		UserID: parsed, UnreadOnly: unreadOnly,
	})
	if err != nil {
		return 0, translate(err, "count notifications")
	}
	return n, nil
}

// CountUnread is what the sidebar badge reads.
func (r *Repository) CountUnread(ctx context.Context, userID string) (int64, error) {
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return 0, apperr.Internal(fmt.Errorf("notification user id: %w", err))
	}
	n, err := r.q(ctx).CountUnreadNotifications(ctx, parsed)
	if err != nil {
		return 0, translate(err, "count unread notifications")
	}
	return n, nil
}

// MarkRead marks one notification read, scoped to userID so one member can
// never mark another's notification read via a guessed id. A row count of
// zero — already read, someone else's, or a nonexistent id — is not
// reported as an error: from the caller's point of view "this notification
// is not unread" is already true either way, and distinguishing "already
// read" from "not yours" would tell an attacker which ids exist.
func (r *Repository) MarkRead(ctx context.Context, id, userID string) error {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return apperr.Invalid("INVALID_ARGUMENT", "id must be a UUID", apperr.Detail{Field: "id", Reason: "not a UUID"})
	}
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return apperr.Internal(fmt.Errorf("notification user id: %w", err))
	}
	if _, err := r.q(ctx).MarkNotificationRead(ctx, db.MarkNotificationReadParams{ID: parsedID, UserID: parsedUserID}); err != nil {
		return translate(err, "mark notification read")
	}
	return nil
}

// MarkAllRead marks every one of userID's unread notifications read.
func (r *Repository) MarkAllRead(ctx context.Context, userID string) error {
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return apperr.Internal(fmt.Errorf("notification user id: %w", err))
	}
	if _, err := r.q(ctx).MarkAllNotificationsRead(ctx, parsed); err != nil {
		return translate(err, "mark all notifications read")
	}
	return nil
}

func translate(err error, op string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("NOTIFICATION_NOT_FOUND", "notification not found")
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == codeForeignKeyViolation {
		return apperr.Conflict("REFERENCE_CONFLICT", "the request references a resource that does not exist")
	}
	return apperr.Internal(fmt.Errorf("%s: %w", op, err))
}
