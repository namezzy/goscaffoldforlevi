// Package userpg is the PostgreSQL implementation of the user repository.
//
// It is the only place in the user feature that knows about pgx, SQL error
// codes, or the shape of the generated code. Everything above it sees domain
// types and apperr values.
package userpg

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/go-enterprise/internal/apperr"
	"github.com/example/go-enterprise/internal/auth"
	"github.com/example/go-enterprise/internal/postgres"
	"github.com/example/go-enterprise/internal/postgres/db"
	"github.com/example/go-enterprise/internal/user"
)

// PostgreSQL error codes this repository interprets.
// https://www.postgresql.org/docs/current/errcodes-appendix.html
const (
	codeUniqueViolation     = "23505"
	codeForeignKeyViolation = "23503"
	codeCheckViolation      = "23514"
)

// Repository stores users.
type Repository struct {
	pool *pgxpool.Pool
}

// New returns a Repository backed by pool.
func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// q returns a querier bound to the ambient transaction if there is one. Every
// method goes through it so that a repository call made inside TxManager.WithTx
// participates in that transaction instead of silently escaping it.
func (r *Repository) q(ctx context.Context) *db.Queries {
	return db.New(postgres.Querier(ctx, r.pool))
}

// Create inserts a user.
func (r *Repository) Create(ctx context.Context, u user.User) (user.User, error) {
	row, err := r.q(ctx).CreateUser(ctx, db.CreateUserParams{
		ID:           u.ID.UUID(),
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Name:         u.Name,
		Status:       u.Status.String(),
		Role:         u.Role.String(),
	})
	if err != nil {
		return user.User{}, translate(err, "create user")
	}
	return toDomain(row), nil
}

// GetByID loads a user by primary key.
func (r *Repository) GetByID(ctx context.Context, id user.ID) (user.User, error) {
	row, err := r.q(ctx).GetUserByID(ctx, id.UUID())
	if err != nil {
		return user.User{}, translate(err, "get user by id")
	}
	return toDomain(row), nil
}

// GetByEmail loads a user by their normalised email address.
func (r *Repository) GetByEmail(ctx context.Context, email string) (user.User, error) {
	row, err := r.q(ctx).GetUserByEmail(ctx, email)
	if err != nil {
		return user.User{}, translate(err, "get user by email")
	}
	return toDomain(row), nil
}

// List returns one page of users in the order fixed by ADR-0004.
func (r *Repository) List(ctx context.Context, limit, offset int32) ([]user.User, error) {
	rows, err := r.q(ctx).ListUsers(ctx, db.ListUsersParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, translate(err, "list users")
	}
	out := make([]user.User, len(rows))
	for i, row := range rows {
		out[i] = toDomain(row)
	}
	return out, nil
}

// Count returns the total number of users, for pagination metadata.
func (r *Repository) Count(ctx context.Context) (int64, error) {
	n, err := r.q(ctx).CountUsers(ctx)
	if err != nil {
		return 0, translate(err, "count users")
	}
	return n, nil
}

// Update applies a partial update and returns the stored row.
func (r *Repository) Update(ctx context.Context, id user.ID, p user.Patch) (user.User, error) {
	params := db.UpdateUserParams{ID: id.UUID(), Name: p.Name, PasswordHash: p.PasswordHash}
	if p.Status != nil {
		s := p.Status.String()
		params.Status = &s
	}
	if p.Role != nil {
		role := p.Role.String()
		params.Role = &role
	}

	row, err := r.q(ctx).UpdateUser(ctx, params)
	if err != nil {
		return user.User{}, translate(err, "update user")
	}
	return toDomain(row), nil
}

// Delete removes a user.
//
// The row count is checked rather than ignored: DELETE against a missing row
// succeeds in SQL, and reporting success for a resource that was never there
// would let a client believe it had deleted something.
func (r *Repository) Delete(ctx context.Context, id user.ID) error {
	n, err := r.q(ctx).DeleteUser(ctx, id.UUID())
	if err != nil {
		return translate(err, "delete user")
	}
	if n == 0 {
		return errUserNotFound()
	}
	return nil
}

func toDomain(row db.User) user.User {
	return user.User{
		ID:           user.ID(row.ID),
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		Name:         row.Name,
		Status:       user.Status(row.Status),
		Role:         auth.Role(row.Role),
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

func errUserNotFound() error {
	return apperr.NotFound("USER_NOT_FOUND", "user not found")
}

// translate converts a driver error into an application error.
//
// This is the boundary. Above it nothing knows what "23505" means, and nothing
// has to import pgx in order to check whether a failure was a duplicate. Doing
// this in the service instead would put the driver back into the business
// layer and force every caller to repeat the mapping.
//
// The original error is kept as the cause, so it is still in the logs; only the
// classification and the client-facing message are decided here.
func translate(err error, op string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errUserNotFound()
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case codeUniqueViolation:
			if pgErr.ConstraintName == "users_email_key" {
				return apperr.Wrap(err, apperr.KindConflict, "EMAIL_ALREADY_EXISTS",
					"a user with this email already exists")
			}
			return apperr.Wrap(err, apperr.KindConflict, "CONFLICT",
				"the resource conflicts with an existing one")
		case codeCheckViolation:
			// A check violation means a value got past validation. That is a
			// bug, so it is reported as invalid input to the client but kept
			// as a wrapped cause so the constraint name reaches the logs.
			return apperr.Wrap(err, apperr.KindInvalidArgument, "INVALID_ARGUMENT",
				"the request contains an unacceptable value")
		case codeForeignKeyViolation:
			return apperr.Wrap(err, apperr.KindConflict, "REFERENCE_CONFLICT",
				"the request references a resource that does not exist")
		}
	}

	return apperr.Internal(fmt.Errorf("%s: %w", op, err))
}
