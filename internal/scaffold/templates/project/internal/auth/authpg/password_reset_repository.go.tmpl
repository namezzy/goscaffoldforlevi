package authpg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/db"
)

// PasswordResetRepository stores password reset tokens.
type PasswordResetRepository struct {
	pool *pgxpool.Pool
}

// NewPasswordResetRepository returns a repository backed by pool.
func NewPasswordResetRepository(pool *pgxpool.Pool) *PasswordResetRepository {
	return &PasswordResetRepository{pool: pool}
}

var _ auth.PasswordResetRepository = (*PasswordResetRepository)(nil)

func (r *PasswordResetRepository) q(ctx context.Context) *db.Queries {
	return db.New(postgres.Querier(ctx, r.pool))
}

// Create records a new password reset token.
func (r *PasswordResetRepository) Create(ctx context.Context, t auth.NewPasswordResetToken) error {
	id, err := uuid.Parse(t.ID)
	if err != nil {
		return apperr.Internal(fmt.Errorf("create password reset token: bad id: %w", err))
	}
	userID, err := uuid.Parse(t.UserID)
	if err != nil {
		return apperr.Internal(fmt.Errorf("create password reset token: bad user id: %w", err))
	}
	if _, err := r.q(ctx).CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
		ID: id, UserID: userID, TokenHash: t.TokenHash, ExpiresAt: t.ExpiresAt,
	}); err != nil {
		return translatePasswordReset(err, "create password reset token")
	}
	return nil
}

// GetByTokenHash loads and locks a token by its digest.
func (r *PasswordResetRepository) GetByTokenHash(ctx context.Context, hash []byte) (auth.PasswordResetToken, error) {
	row, err := r.q(ctx).GetPasswordResetTokenByHash(ctx, hash)
	if err != nil {
		return auth.PasswordResetToken{}, translatePasswordReset(err, "get password reset token")
	}
	return toPasswordResetDomain(row), nil
}

// MarkUsed marks one token as consumed. The bool reports whether anything
// was actually marked — false means it was already used, or never existed.
func (r *PasswordResetRepository) MarkUsed(ctx context.Context, id string) (bool, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return false, apperr.Internal(fmt.Errorf("mark password reset token used: %w", err))
	}
	n, err := r.q(ctx).MarkPasswordResetTokenUsed(ctx, parsed)
	if err != nil {
		return false, translatePasswordReset(err, "mark password reset token used")
	}
	return n > 0, nil
}

// InvalidatePendingForUser removes every unconsumed token the user holds.
func (r *PasswordResetRepository) InvalidatePendingForUser(ctx context.Context, userID string) (int64, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return 0, apperr.Internal(fmt.Errorf("invalidate pending password reset tokens: %w", err))
	}
	n, err := r.q(ctx).InvalidatePendingPasswordResetTokensForUser(ctx, id)
	if err != nil {
		return 0, translatePasswordReset(err, "invalidate pending password reset tokens")
	}
	return n, nil
}

func toPasswordResetDomain(row db.PasswordResetToken) auth.PasswordResetToken {
	return auth.PasswordResetToken{
		ID: row.ID.String(), UserID: row.UserID.String(), TokenHash: row.TokenHash,
		ExpiresAt: row.ExpiresAt, UsedAt: row.UsedAt, CreatedAt: row.CreatedAt,
	}
}

func translatePasswordReset(err error, op string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("PASSWORD_RESET_TOKEN_NOT_FOUND", "password reset token not found")
	}
	return apperr.Internal(fmt.Errorf("%s: %w", op, err))
}
