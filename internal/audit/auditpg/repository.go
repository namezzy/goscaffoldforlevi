// Package auditpg is the PostgreSQL implementation of audit.Store.
package auditpg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/db"
)

// Repository stores audit events.
type Repository struct {
	pool *pgxpool.Pool
}

// New returns a Repository backed by pool.
func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) q(ctx context.Context) *db.Queries {
	return db.New(postgres.Querier(ctx, r.pool))
}

// Insert writes one audit record.
//
// It participates in the caller's transaction via postgres.Querier the same
// way every other repository in this codebase does — that is what lets
// user.Service (and org.Service, invite.Service) write the business change
// and its audit record as one commit.
func (r *Repository) Insert(ctx context.Context, rec audit.Record) error {
	id, err := uuid.Parse(rec.ID)
	if err != nil {
		return apperr.Internal(fmt.Errorf("audit record id: %w", err))
	}
	var actorID *uuid.UUID
	if rec.ActorID != nil {
		parsed, err := uuid.Parse(*rec.ActorID)
		if err != nil {
			return apperr.Internal(fmt.Errorf("audit actor id: %w", err))
		}
		actorID = &parsed
	}
	var requestID *string
	if rec.RequestID != "" {
		requestID = &rec.RequestID
	}

	_, err = r.q(ctx).InsertAuditEvent(ctx, db.InsertAuditEventParams{
		ID:         id,
		ActorID:    actorID,
		Action:     rec.Action,
		TargetType: rec.TargetType,
		TargetID:   rec.TargetID,
		Result:     string(rec.Result),
		RequestID:  requestID,
		BeforeData: rec.Before,
		AfterData:  rec.After,
	})
	if err != nil {
		return translate(err, "insert audit event")
	}
	return nil
}

// List returns a filtered, paginated page of audit events, most recent first.
func (r *Repository) List(ctx context.Context, f audit.Filter, limit, offset int32) ([]audit.Record, error) {
	params := db.ListAuditEventsFilteredParams{
		Limit: limit, Offset: offset,
		FromTime: f.From, ToTime: f.To,
		Action: f.Action, TargetType: f.TargetType, TargetID: f.TargetID,
	}
	if f.ActorID != nil {
		id, err := uuid.Parse(*f.ActorID)
		if err != nil {
			return nil, apperr.Invalid("INVALID_ACTOR_ID", "actor id must be a UUID")
		}
		params.ActorID = &id
	}
	if f.Result != nil {
		s := string(*f.Result)
		params.Result = &s
	}

	rows, err := r.q(ctx).ListAuditEventsFiltered(ctx, params)
	if err != nil {
		return nil, translate(err, "list audit events")
	}
	out := make([]audit.Record, len(rows))
	for i, row := range rows {
		out[i] = toDomain(row)
	}
	return out, nil
}

// Count returns the total number of audit events matching f, for pagination
// metadata.
func (r *Repository) Count(ctx context.Context, f audit.Filter) (int64, error) {
	params := db.CountAuditEventsFilteredParams{
		FromTime: f.From, ToTime: f.To,
		Action: f.Action, TargetType: f.TargetType, TargetID: f.TargetID,
	}
	if f.ActorID != nil {
		id, err := uuid.Parse(*f.ActorID)
		if err != nil {
			return 0, apperr.Invalid("INVALID_ACTOR_ID", "actor id must be a UUID")
		}
		params.ActorID = &id
	}
	if f.Result != nil {
		s := string(*f.Result)
		params.Result = &s
	}

	n, err := r.q(ctx).CountAuditEventsFiltered(ctx, params)
	if err != nil {
		return 0, translate(err, "count audit events")
	}
	return n, nil
}

// Get loads one audit event by ID.
func (r *Repository) Get(ctx context.Context, id string) (audit.Record, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return audit.Record{}, apperr.Invalid("INVALID_AUDIT_ID", "audit event id must be a UUID")
	}
	row, err := r.q(ctx).GetAuditEventByID(ctx, parsed)
	if err != nil {
		return audit.Record{}, translate(err, "get audit event")
	}
	return toDomain(row), nil
}

func toDomain(row db.AuditEvent) audit.Record {
	rec := audit.Record{
		ID:         row.ID.String(),
		Action:     row.Action,
		TargetType: row.TargetType,
		TargetID:   row.TargetID,
		Result:     audit.Result(row.Result),
		Before:     row.BeforeData,
		After:      row.AfterData,
		CreatedAt:  row.CreatedAt,
	}
	if row.ActorID != nil {
		s := row.ActorID.String()
		rec.ActorID = &s
	}
	if row.RequestID != nil {
		rec.RequestID = *row.RequestID
	}
	return rec
}

func errAuditEventNotFound() error {
	return apperr.NotFound("AUDIT_EVENT_NOT_FOUND", "audit event not found")
}

func translate(err error, op string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errAuditEventNotFound()
	}
	return apperr.Internal(fmt.Errorf("%s: %w", op, err))
}
