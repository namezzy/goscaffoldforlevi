// Package incidentpg is the PostgreSQL implementation of the incident
// repository — both incident.Repository (Service's port) and
// incident.PollRepository (Watcher's port), the same way messagingpg.Store
// backs both messaging.AdminStore and messaging.OutboxStore.
package incidentpg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/incident"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/db"
)

// Store persists incidents.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a Store backed by pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) q(ctx context.Context) *db.Queries {
	return db.New(postgres.Querier(ctx, s.pool))
}

func errIncidentNotFound() error {
	return apperr.NotFound("INCIDENT_NOT_FOUND", "incident not found")
}

func translate(err error, op string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errIncidentNotFound()
	}
	return apperr.Internal(fmt.Errorf("%s: %w", op, err))
}

func parseUUID(id string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.UUID{}, errIncidentNotFound()
	}
	return parsed, nil
}

func toDomain(row db.Incident) incident.Incident {
	out := incident.Incident{
		ID:                row.ID.String(),
		Fingerprint:       row.Fingerprint,
		Severity:          incident.Severity(row.Severity),
		Status:            incident.Status(row.Status),
		DeadLetterID:      row.DeadLetterID.String(),
		EventID:           row.EventID.String(),
		EventType:         row.EventType,
		Consumer:          row.Consumer,
		LastError:         row.LastError,
		OccurrenceCount:   row.OccurrenceCount,
		FirstSeenAt:       row.FirstSeenAt,
		LastSeenAt:        row.LastSeenAt,
		AcknowledgedAt:    row.AcknowledgedAt,
		ResolvedAt:        row.ResolvedAt,
		ReplayRequestedAt: row.ReplayRequestedAt,
		ReplayResultAt:    row.ReplayResultAt,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
	if row.OwnerID != nil {
		s := row.OwnerID.String()
		out.OwnerID = &s
	}
	if row.AcknowledgedBy != nil {
		s := row.AcknowledgedBy.String()
		out.AcknowledgedBy = &s
	}
	if row.ResolvedBy != nil {
		s := row.ResolvedBy.String()
		out.ResolvedBy = &s
	}
	if row.ReplayStatus != nil {
		rs := incident.ReplayStatus(*row.ReplayStatus)
		out.ReplayStatus = &rs
	}
	if row.ReplayEventID != nil {
		s := row.ReplayEventID.String()
		out.ReplayEventID = &s
	}
	return out
}

// --- incident.Repository (Service's port) ---

func filterParams(f incident.ListFilter) (status, severity *string, ownerID *uuid.UUID, search *string, err error) {
	if f.Status != nil {
		s := string(*f.Status)
		status = &s
	}
	if f.Severity != nil {
		s := string(*f.Severity)
		severity = &s
	}
	if f.OwnerID != nil {
		id, perr := uuid.Parse(*f.OwnerID)
		if perr != nil {
			return nil, nil, nil, nil, apperr.Invalid("INVALID_QUERY_PARAM", "owner_id must be a UUID",
				apperr.Detail{Field: "owner_id", Reason: "not a UUID"})
		}
		ownerID = &id
	}
	if f.Search != "" {
		search = &f.Search
	}
	return status, severity, ownerID, search, nil
}

// ListFiltered returns a page of incidents matching f.
func (s *Store) ListFiltered(ctx context.Context, f incident.ListFilter, limit, offset int32) ([]incident.Incident, error) {
	status, severity, ownerID, search, err := filterParams(f)
	if err != nil {
		return nil, err
	}
	rows, err := s.q(ctx).ListIncidentsFiltered(ctx, db.ListIncidentsFilteredParams{
		Limit: limit, Offset: offset, Status: status, Severity: severity, OwnerID: ownerID, Search: search,
	})
	if err != nil {
		return nil, translate(err, "list incidents")
	}
	out := make([]incident.Incident, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

// CountFiltered counts incidents matching f.
func (s *Store) CountFiltered(ctx context.Context, f incident.ListFilter) (int64, error) {
	status, severity, ownerID, search, err := filterParams(f)
	if err != nil {
		return 0, err
	}
	n, err := s.q(ctx).CountIncidentsFiltered(ctx, db.CountIncidentsFilteredParams{
		Status: status, Severity: severity, OwnerID: ownerID, Search: search,
	})
	if err != nil {
		return 0, translate(err, "count incidents")
	}
	return n, nil
}

// CountOpenBySeverity backs the exception center's summary strip.
func (s *Store) CountOpenBySeverity(ctx context.Context) (map[incident.Severity]int64, error) {
	rows, err := s.q(ctx).CountOpenIncidentsBySeverity(ctx)
	if err != nil {
		return nil, translate(err, "count open incidents by severity")
	}
	out := make(map[incident.Severity]int64, len(rows))
	for _, row := range rows {
		out[incident.Severity(row.Severity)] = row.Total
	}
	return out, nil
}

// Get loads an incident by ID.
func (s *Store) Get(ctx context.Context, id string) (incident.Incident, error) {
	parsed, err := parseUUID(id)
	if err != nil {
		return incident.Incident{}, err
	}
	row, err := s.q(ctx).GetIncident(ctx, parsed)
	if err != nil {
		return incident.Incident{}, translate(err, "get incident")
	}
	return toDomain(row), nil
}

// GetForUpdate loads an incident row-locked, for a mutating action.
func (s *Store) GetForUpdate(ctx context.Context, id string) (incident.Incident, error) {
	parsed, err := parseUUID(id)
	if err != nil {
		return incident.Incident{}, err
	}
	row, err := s.q(ctx).GetIncidentForUpdate(ctx, parsed)
	if err != nil {
		return incident.Incident{}, translate(err, "get incident for update")
	}
	return toDomain(row), nil
}

// SetSeverity re-triages an incident.
func (s *Store) SetSeverity(ctx context.Context, id string, severity incident.Severity) (incident.Incident, error) {
	if _, err := s.GetForUpdate(ctx, id); err != nil {
		return incident.Incident{}, err
	}
	parsed, _ := parseUUID(id)
	if err := s.q(ctx).SetIncidentSeverity(ctx, db.SetIncidentSeverityParams{ID: parsed, Severity: string(severity)}); err != nil {
		return incident.Incident{}, translate(err, "set incident severity")
	}
	return s.Get(ctx, id)
}

// AssignOwner sets or clears the incident's owner.
func (s *Store) AssignOwner(ctx context.Context, id string, ownerID *string) (incident.Incident, error) {
	if _, err := s.GetForUpdate(ctx, id); err != nil {
		return incident.Incident{}, err
	}
	parsed, _ := parseUUID(id)
	var ownerUUID *uuid.UUID
	if ownerID != nil {
		u, err := uuid.Parse(*ownerID)
		if err != nil {
			return incident.Incident{}, apperr.Invalid("INVALID_ARGUMENT", "owner_id must be a UUID",
				apperr.Detail{Field: "owner_id", Reason: "not a UUID"})
		}
		ownerUUID = &u
	}
	if err := s.q(ctx).AssignIncidentOwner(ctx, db.AssignIncidentOwnerParams{ID: parsed, OwnerID: ownerUUID}); err != nil {
		return incident.Incident{}, translate(err, "assign incident owner")
	}
	return s.Get(ctx, id)
}

// Acknowledge marks an open incident seen. ok is false when the incident was
// not open, matching incident.Repository's doc comment.
func (s *Store) Acknowledge(ctx context.Context, id, actorID string) (incident.Incident, bool, error) {
	parsed, err := parseUUID(id)
	if err != nil {
		return incident.Incident{}, false, err
	}
	actor, err := uuid.Parse(actorID)
	if err != nil {
		return incident.Incident{}, false, apperr.Internal(fmt.Errorf("actor id: %w", err))
	}
	rows, err := s.q(ctx).AcknowledgeIncident(ctx, db.AcknowledgeIncidentParams{ID: parsed, AcknowledgedBy: &actor})
	if err != nil {
		return incident.Incident{}, false, translate(err, "acknowledge incident")
	}
	if rows == 0 {
		// Zero rows means either no such incident, or one that exists but is
		// not open — the conditional UPDATE cannot tell the two apart on its
		// own, so a lookup decides which: a missing incident must answer 404,
		// never the misleading 409 that "not open" implies.
		if _, err := s.Get(ctx, id); err != nil {
			return incident.Incident{}, false, err
		}
		return incident.Incident{}, false, nil
	}
	updated, err := s.Get(ctx, id)
	return updated, true, err
}

// Resolve closes the incident's triage. ok is false when it was already
// resolved.
func (s *Store) Resolve(ctx context.Context, id, actorID string) (incident.Incident, bool, error) {
	parsed, err := parseUUID(id)
	if err != nil {
		return incident.Incident{}, false, err
	}
	actor, err := uuid.Parse(actorID)
	if err != nil {
		return incident.Incident{}, false, apperr.Internal(fmt.Errorf("actor id: %w", err))
	}
	rows, err := s.q(ctx).ResolveIncident(ctx, db.ResolveIncidentParams{ID: parsed, ResolvedBy: &actor})
	if err != nil {
		return incident.Incident{}, false, translate(err, "resolve incident")
	}
	if rows == 0 {
		// See Acknowledge's identical comment: zero rows is ambiguous
		// between "no such incident" and "already resolved" without this
		// lookup.
		if _, err := s.Get(ctx, id); err != nil {
			return incident.Incident{}, false, err
		}
		return incident.Incident{}, false, nil
	}
	updated, err := s.Get(ctx, id)
	return updated, true, err
}

// RequestReplay records that a replay was submitted for this incident.
func (s *Store) RequestReplay(ctx context.Context, id, replayEventID string) (incident.Incident, error) {
	parsed, err := parseUUID(id)
	if err != nil {
		return incident.Incident{}, err
	}
	eventID, err := uuid.Parse(replayEventID)
	if err != nil {
		return incident.Incident{}, apperr.Internal(fmt.Errorf("request incident replay: event id %q: %w", replayEventID, err))
	}
	if err := s.q(ctx).RequestIncidentReplay(ctx, db.RequestIncidentReplayParams{
		ID: parsed, ReplayEventID: &eventID,
	}); err != nil {
		return incident.Incident{}, translate(err, "request incident replay")
	}
	return s.Get(ctx, id)
}

// --- incident.PollRepository (Watcher's port) ---

// ClaimUntriaged takes the oldest dead letter not yet triaged and holds it
// for the rest of the transaction, or ok=false if there is none.
func (s *Store) ClaimUntriaged(ctx context.Context) (incident.DeadLetterOccurrence, bool, error) {
	row, err := s.q(ctx).ClaimUntriagedDeadLetter(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return incident.DeadLetterOccurrence{}, false, nil
		}
		return incident.DeadLetterOccurrence{}, false, translate(err, "claim untriaged dead letter")
	}
	return incident.DeadLetterOccurrence{
		ID: row.ID.String(), EventID: row.EventID.String(), EventType: row.EventType,
		Consumer: row.Consumer, LastError: row.LastError, FailedAt: row.FailedAt,
	}, true, nil
}

// MarkTriaged records that this dead letter has become, or been merged
// into, an incident.
func (s *Store) MarkTriaged(ctx context.Context, deadLetterID string) error {
	parsed, err := uuid.Parse(deadLetterID)
	if err != nil {
		return apperr.Internal(fmt.Errorf("mark dead letter triaged: id %q: %w", deadLetterID, err))
	}
	if err := s.q(ctx).MarkDeadLetterTriaged(ctx, parsed); err != nil {
		return translate(err, "mark dead letter triaged")
	}
	return nil
}

// LockOpenByFingerprint finds the merge target, row-locked, or ok=false if
// no non-resolved incident shares this fingerprint.
func (s *Store) LockOpenByFingerprint(ctx context.Context, fingerprint string) (incident.Incident, bool, error) {
	row, err := s.q(ctx).GetOpenIncidentByFingerprint(ctx, fingerprint)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return incident.Incident{}, false, nil
		}
		return incident.Incident{}, false, translate(err, "lock open incident by fingerprint")
	}
	return toDomain(row), true, nil
}

// Create opens a new incident.
func (s *Store) Create(ctx context.Context, in incident.NewIncident) (incident.Incident, error) {
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return incident.Incident{}, apperr.Internal(fmt.Errorf("incident id: %w", err))
	}
	deadLetterID, err := uuid.Parse(in.DeadLetterID)
	if err != nil {
		return incident.Incident{}, apperr.Internal(fmt.Errorf("dead letter id: %w", err))
	}
	eventID, err := uuid.Parse(in.EventID)
	if err != nil {
		return incident.Incident{}, apperr.Internal(fmt.Errorf("event id: %w", err))
	}
	row, err := s.q(ctx).CreateIncident(ctx, db.CreateIncidentParams{
		ID: id, Fingerprint: in.Fingerprint, DeadLetterID: deadLetterID, EventID: eventID,
		EventType: in.EventType, Consumer: in.Consumer, LastError: in.LastError, FirstSeenAt: in.FirstSeenAt,
	})
	if err != nil {
		return incident.Incident{}, translate(err, "create incident")
	}
	return toDomain(row), nil
}

// MergeOccurrence records a repeat failure onto an existing incident.
// occurredAt is this occurrence's own failed_at, not necessarily the newest
// one: the query decides which occurrence the incident ends up describing.
func (s *Store) MergeOccurrence(ctx context.Context, id string, occurredAt time.Time, deadLetterID, eventID, lastError string) error {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return apperr.Internal(fmt.Errorf("incident id: %w", err))
	}
	parsedDeadLetterID, err := uuid.Parse(deadLetterID)
	if err != nil {
		return apperr.Internal(fmt.Errorf("dead letter id: %w", err))
	}
	parsedEventID, err := uuid.Parse(eventID)
	if err != nil {
		return apperr.Internal(fmt.Errorf("event id: %w", err))
	}
	if err := s.q(ctx).MergeIncidentOccurrence(ctx, db.MergeIncidentOccurrenceParams{
		ID: parsedID, OccurredAt: occurredAt, DeadLetterID: parsedDeadLetterID, EventID: parsedEventID, LastError: lastError,
	}); err != nil {
		return translate(err, "merge incident occurrence")
	}
	return nil
}

// ListPendingReplay returns incidents whose last requested replay has not
// yet been resolved.
func (s *Store) ListPendingReplay(ctx context.Context, limit int32) ([]incident.Incident, error) {
	rows, err := s.q(ctx).ListPendingReplayIncidents(ctx, limit)
	if err != nil {
		return nil, translate(err, "list pending replay incidents")
	}
	out := make([]incident.Incident, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

// ExistsProcessedSince is the real "did the replay succeed" check.
func (s *Store) ExistsProcessedSince(ctx context.Context, eventID, consumer string, since time.Time) (bool, error) {
	parsed, err := uuid.Parse(eventID)
	if err != nil {
		return false, apperr.Internal(fmt.Errorf("event id: %w", err))
	}
	ok, err := s.q(ctx).ExistsProcessedEventSince(ctx, db.ExistsProcessedEventSinceParams{
		EventID: parsed, Consumer: consumer, ProcessedAt: since,
	})
	if err != nil {
		return false, translate(err, "exists processed event since")
	}
	return ok, nil
}

// ExistsDeadLetterSince is the matching "did the replay fail again" check —
// see queries/incident.sql's ExistsDeadLetterSince.
func (s *Store) ExistsDeadLetterSince(ctx context.Context, eventID, consumer string, since time.Time) (bool, error) {
	parsed, err := uuid.Parse(eventID)
	if err != nil {
		return false, apperr.Internal(fmt.Errorf("event id: %w", err))
	}
	ok, err := s.q(ctx).ExistsDeadLetterSince(ctx, db.ExistsDeadLetterSinceParams{
		EventID: parsed, Consumer: consumer, FailedAt: since,
	})
	if err != nil {
		return false, translate(err, "exists dead letter since")
	}
	return ok, nil
}

// ResolveReplay records the real outcome of the last requested replay.
func (s *Store) ResolveReplay(ctx context.Context, id string, result incident.ReplayStatus) (incident.Incident, error) {
	parsed, err := parseUUID(id)
	if err != nil {
		return incident.Incident{}, err
	}
	status := string(result)
	if err := s.q(ctx).ResolveIncidentReplay(ctx, db.ResolveIncidentReplayParams{ID: parsed, ReplayStatus: &status}); err != nil {
		return incident.Incident{}, translate(err, "resolve incident replay")
	}
	return s.Get(ctx, id)
}
