// Package authpg is the PostgreSQL implementation of the refresh session
// repository. It is the only place in the auth feature that knows about pgx.
package authpg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/go-enterprise/internal/apperr"
	"github.com/example/go-enterprise/internal/auth"
	"github.com/example/go-enterprise/internal/postgres"
	"github.com/example/go-enterprise/internal/postgres/db"
)

const (
	codeUniqueViolation     = "23505"
	codeForeignKeyViolation = "23503"
)

// SessionRepository stores refresh sessions.
type SessionRepository struct {
	pool *pgxpool.Pool
}

// NewSessionRepository returns a repository backed by pool.
func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

var _ auth.SessionRepository = (*SessionRepository)(nil)

func (r *SessionRepository) q(ctx context.Context) *db.Queries {
	return db.New(postgres.Querier(ctx, r.pool))
}

// Create records a new session.
func (r *SessionRepository) Create(ctx context.Context, s auth.NewSession) (auth.Session, error) {
	userID, err := uuid.Parse(s.UserID)
	if err != nil {
		return auth.Session{}, apperr.Internal(fmt.Errorf("create refresh session: bad user id: %w", err))
	}

	var parent *uuid.UUID
	if s.ParentID != nil {
		p := s.ParentID.UUID()
		parent = &p
	}

	row, err := r.q(ctx).CreateRefreshSession(ctx, db.CreateRefreshSessionParams{
		ID:        s.ID.UUID(),
		UserID:    userID,
		TokenHash: s.TokenHash,
		FamilyID:  s.FamilyID.UUID(),
		ParentID:  parent,
		ExpiresAt: s.ExpiresAt,
		UserAgent: s.UserAgent,
		Ip:        s.IP,
	})
	if err != nil {
		return auth.Session{}, translate(err, "create refresh session")
	}
	return toDomain(row), nil
}

// GetByTokenHash loads and locks a session by its token digest.
func (r *SessionRepository) GetByTokenHash(ctx context.Context, hash []byte) (auth.Session, error) {
	row, err := r.q(ctx).GetRefreshSessionByTokenHash(ctx, hash)
	if err != nil {
		return auth.Session{}, translate(err, "get refresh session")
	}
	return toDomain(row), nil
}

// Revoke marks one session as revoked.
//
// A zero row count is not an error: the statement only touches rows that are
// still active, so re-revoking an already revoked session is a no-op by design.
func (r *SessionRepository) Revoke(ctx context.Context, id auth.SessionID) error {
	if _, err := r.q(ctx).RevokeRefreshSession(ctx, id.UUID()); err != nil {
		return translate(err, "revoke refresh session")
	}
	return nil
}

// RevokeFamily revokes every active session descended from one login and
// returns how many were affected, which is the number the reuse-detection log
// line reports.
func (r *SessionRepository) RevokeFamily(ctx context.Context, id auth.FamilyID) (int64, error) {
	n, err := r.q(ctx).RevokeRefreshFamily(ctx, id.UUID())
	if err != nil {
		return 0, translate(err, "revoke refresh family")
	}
	return n, nil
}

// DeleteExpired removes sessions past their expiry. It is used by the cleanup
// job; nothing in the request path depends on it, because expiry is enforced by
// comparison rather than by the row's absence.
func (r *SessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	n, err := r.q(ctx).DeleteExpiredRefreshSessions(ctx)
	if err != nil {
		return 0, translate(err, "delete expired refresh sessions")
	}
	return n, nil
}

func toDomain(row db.RefreshSession) auth.Session {
	s := auth.Session{
		ID:        auth.SessionID(row.ID),
		UserID:    row.UserID.String(),
		TokenHash: row.TokenHash,
		FamilyID:  auth.FamilyID(row.FamilyID),
		ExpiresAt: row.ExpiresAt,
		RevokedAt: row.RevokedAt,
		CreatedAt: row.CreatedAt,
		UserAgent: row.UserAgent,
		IP:        row.Ip,
	}
	if row.ParentID != nil {
		p := auth.SessionID(*row.ParentID)
		s.ParentID = &p
	}
	return s
}

// translate converts driver errors into application errors. As in userpg, this
// is the only place SQL error codes are interpreted.
func translate(err error, op string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		// A token that matches no row is indistinguishable from an invalid one
		// as far as the client is concerned, but the classification has to be
		// NotFound here so the service can decide that for itself.
		return apperr.NotFound("SESSION_NOT_FOUND", "session not found")
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case codeUniqueViolation:
			// Two sessions cannot share a token digest. Hitting this means the
			// CSPRNG produced a collision or a token was reinserted, both of
			// which are bugs rather than client errors.
			return apperr.Wrap(err, apperr.KindConflict, "SESSION_CONFLICT", "session already exists")
		case codeForeignKeyViolation:
			return apperr.Wrap(err, apperr.KindConflict, "REFERENCE_CONFLICT",
				"the request references a resource that does not exist")
		}
	}

	return apperr.Internal(fmt.Errorf("%s: %w", op, err))
}
