// Package invitepg is the PostgreSQL implementation of the invite repository.
package invitepg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/invite"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/db"
)

const (
	codeForeignKeyViolation = "23503"
	codeCheckViolation      = "23514"
)

// Repository stores invitations.
type Repository struct {
	pool *pgxpool.Pool
}

// New returns a Repository backed by pool.
func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) q(ctx context.Context) *db.Queries {
	return db.New(postgres.Querier(ctx, r.pool))
}

// Create inserts an invitation. tokenHash is stored; the plaintext token
// that produced it is not — see invite.Repository's doc comment.
func (r *Repository) Create(ctx context.Context, inv invite.Invitation, tokenHash []byte) (invite.Invitation, error) {
	id, err := uuid.Parse(inv.ID)
	if err != nil {
		return invite.Invitation{}, apperr.Internal(fmt.Errorf("invitation id: %w", err))
	}
	invitedBy, err := uuid.Parse(inv.InvitedBy)
	if err != nil {
		return invite.Invitation{}, apperr.Internal(fmt.Errorf("invited_by: %w", err))
	}
	departmentID, err := parseOptionalUUID(inv.DepartmentID)
	if err != nil {
		return invite.Invitation{}, apperr.Invalid("INVALID_ARGUMENT", "department_id must be a UUID",
			apperr.Detail{Field: "department_id", Reason: "not a UUID"})
	}

	row, err := r.q(ctx).CreateInvitation(ctx, db.CreateInvitationParams{
		ID: id, Email: inv.Email, DepartmentID: departmentID, Role: inv.Role.String(),
		TokenHash: tokenHash, ExpiresAt: inv.ExpiresAt, InvitedBy: invitedBy,
	})
	if err != nil {
		return invite.Invitation{}, translate(err, "create invitation")
	}
	return invite.Invitation{
		ID: row.ID.String(), Email: row.Email, DepartmentID: uuidPtrToString(row.DepartmentID),
		Role: auth.Role(row.Role), Status: invite.Status(row.Status), InvitedBy: row.InvitedBy.String(),
		AcceptedUserID: uuidPtrToString(row.AcceptedUserID),
		ExpiresAt:      row.ExpiresAt, AcceptedAt: row.AcceptedAt, RevokedAt: row.RevokedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

// GetByID loads an invitation by primary key.
func (r *Repository) GetByID(ctx context.Context, id string) (invite.Invitation, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return invite.Invitation{}, errInvitationNotFound()
	}
	row, err := r.q(ctx).GetInvitationByID(ctx, parsed)
	if err != nil {
		return invite.Invitation{}, translate(err, "get invitation")
	}
	return invite.Invitation{
		ID: row.ID.String(), Email: row.Email, DepartmentID: uuidPtrToString(row.DepartmentID),
		Role: auth.Role(row.Role), Status: invite.Status(row.Status), InvitedBy: row.InvitedBy.String(),
		AcceptedUserID: uuidPtrToString(row.AcceptedUserID),
		ExpiresAt:      row.ExpiresAt, AcceptedAt: row.AcceptedAt, RevokedAt: row.RevokedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

// GetByTokenHash loads an invitation by its token digest.
func (r *Repository) GetByTokenHash(ctx context.Context, tokenHash []byte) (invite.Invitation, error) {
	row, err := r.q(ctx).GetInvitationByTokenHash(ctx, tokenHash)
	if err != nil {
		return invite.Invitation{}, translate(err, "get invitation by token")
	}
	return invite.Invitation{
		ID: row.ID.String(), Email: row.Email, DepartmentID: uuidPtrToString(row.DepartmentID),
		Role: auth.Role(row.Role), Status: invite.Status(row.Status), InvitedBy: row.InvitedBy.String(),
		AcceptedUserID: uuidPtrToString(row.AcceptedUserID),
		ExpiresAt:      row.ExpiresAt, AcceptedAt: row.AcceptedAt, RevokedAt: row.RevokedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

// ListFiltered returns one page of the invitation management list.
func (r *Repository) ListFiltered(ctx context.Context, f invite.ListFilter, limit, offset int32) ([]invite.Invitation, error) {
	rows, err := r.q(ctx).ListInvitationsFiltered(ctx, db.ListInvitationsFilteredParams{
		Limit: limit, Offset: offset, Status: statusFilter(f.Status), Email: nullableString(f.Email),
	})
	if err != nil {
		return nil, translate(err, "list invitations")
	}
	out := make([]invite.Invitation, len(rows))
	for i, row := range rows {
		out[i] = invite.Invitation{
			ID: row.ID.String(), Email: row.Email, DepartmentID: uuidPtrToString(row.DepartmentID),
			Role: auth.Role(row.Role), Status: invite.Status(row.Status), InvitedBy: row.InvitedBy.String(),
			AcceptedUserID: uuidPtrToString(row.AcceptedUserID),
			ExpiresAt:      row.ExpiresAt, AcceptedAt: row.AcceptedAt, RevokedAt: row.RevokedAt,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}
	}
	return out, nil
}

// CountFiltered returns the total number of invitations matching f.
func (r *Repository) CountFiltered(ctx context.Context, f invite.ListFilter) (int64, error) {
	n, err := r.q(ctx).CountInvitationsFiltered(ctx, db.CountInvitationsFilteredParams{
		Status: statusFilter(f.Status), Email: nullableString(f.Email),
	})
	if err != nil {
		return 0, translate(err, "count invitations")
	}
	return n, nil
}

// MarkAccepted flips a pending invitation to accepted. See
// invite.Repository's doc comment for why the affected-row check, not a lock
// taken earlier, is the actual one-time-use guarantee.
func (r *Repository) MarkAccepted(ctx context.Context, id, acceptedUserID string) error {
	invID, err := uuid.Parse(id)
	if err != nil {
		return errInvitationNotFound()
	}
	userID, err := uuid.Parse(acceptedUserID)
	if err != nil {
		return apperr.Internal(fmt.Errorf("accepted user id: %w", err))
	}
	n, err := r.q(ctx).MarkInvitationAccepted(ctx, db.MarkInvitationAcceptedParams{ID: invID, AcceptedUserID: &userID})
	if err != nil {
		return translate(err, "mark invitation accepted")
	}
	if n == 0 {
		return apperr.Conflict("INVITATION_NOT_PENDING", "this invitation is no longer pending")
	}
	return nil
}

// MarkExpired flips a pending invitation to expired.
func (r *Repository) MarkExpired(ctx context.Context, id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return errInvitationNotFound()
	}
	n, err := r.q(ctx).MarkInvitationExpired(ctx, parsed)
	if err != nil {
		return translate(err, "mark invitation expired")
	}
	if n == 0 {
		return apperr.Conflict("INVITATION_NOT_PENDING", "this invitation is no longer pending")
	}
	return nil
}

// Revoke flips a pending invitation to revoked.
func (r *Repository) Revoke(ctx context.Context, id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return errInvitationNotFound()
	}
	n, err := r.q(ctx).RevokeInvitation(ctx, parsed)
	if err != nil {
		return translate(err, "revoke invitation")
	}
	if n == 0 {
		return apperr.Conflict("INVITATION_NOT_PENDING", "this invitation is no longer pending")
	}
	return nil
}

func statusFilter(s *invite.Status) *string {
	if s == nil {
		return nil
	}
	v := string(*s)
	return &v
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func parseOptionalUUID(s *string) (*uuid.UUID, error) {
	if s == nil {
		return nil, nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func uuidPtrToString(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

func errInvitationNotFound() error {
	return apperr.NotFound("INVITATION_NOT_FOUND", "invitation not found")
}

func translate(err error, op string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errInvitationNotFound()
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case codeForeignKeyViolation:
			return apperr.Conflict("REFERENCE_CONFLICT", "the request references a resource that does not exist")
		case codeCheckViolation:
			return apperr.Wrap(err, apperr.KindInvalidArgument, "INVALID_ARGUMENT",
				"the request contains an unacceptable value")
		}
	}

	return apperr.Internal(fmt.Errorf("%s: %w", op, err))
}
