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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/db"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
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
	departmentID, err := parseOptionalUUID(u.DepartmentID)
	if err != nil {
		return user.User{}, apperr.Invalid("INVALID_ARGUMENT", "department_id must be a UUID",
			apperr.Detail{Field: "department_id", Reason: "not a UUID"})
	}
	row, err := r.q(ctx).CreateUser(ctx, db.CreateUserParams{
		ID:           u.ID.UUID(),
		Email:        u.Email,
		Username:     nullableString(u.Username),
		PasswordHash: u.PasswordHash,
		Name:         u.Name,
		Status:       u.Status.String(),
		Role:         u.Role.String(),
		DepartmentID: departmentID,
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

// GetByIDForUpdate loads a user and locks the row for the rest of the
// current transaction — see queries/user.sql's GetUserByIDForUpdate for why
// the guards in Update and Delete depend on the lock, not just a fresh read.
func (r *Repository) GetByIDForUpdate(ctx context.Context, id user.ID) (user.User, error) {
	row, err := r.q(ctx).GetUserByIDForUpdate(ctx, id.UUID())
	if err != nil {
		return user.User{}, translate(err, "get user by id for update")
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

// GetByIdentifier loads a user by normalised email or username, whichever
// matches. The caller has already normalised it; doing so again here would
// hide a mismatch rather than fix it.
func (r *Repository) GetByIdentifier(ctx context.Context, identifier string) (user.User, error) {
	row, err := r.q(ctx).GetUserByIdentifier(ctx, identifier)
	if err != nil {
		return user.User{}, translate(err, "get user by identifier")
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

// ListFiltered returns one page of the member-directory search.
func (r *Repository) ListFiltered(ctx context.Context, f user.ListFilter, limit, offset int32) ([]user.User, error) {
	rows, err := r.q(ctx).ListUsersFiltered(ctx, db.ListUsersFilteredParams{
		Limit: limit, Offset: offset,
		Search: nullableString(f.Search), Role: roleFilter(f.Role), Status: statusFilter(f.Status),
		SortBy: string(f.SortBy), SortDesc: f.SortDesc,
	})
	if err != nil {
		return nil, translate(err, "list filtered users")
	}
	out := make([]user.User, len(rows))
	for i, row := range rows {
		out[i] = toDomain(row)
	}
	return out, nil
}

// CountFiltered returns the total number of users matching f, for pagination
// metadata on the filtered listing.
func (r *Repository) CountFiltered(ctx context.Context, f user.ListFilter) (int64, error) {
	n, err := r.q(ctx).CountUsersFiltered(ctx, db.CountUsersFilteredParams{
		Search: nullableString(f.Search), Role: roleFilter(f.Role), Status: statusFilter(f.Status),
	})
	if err != nil {
		return 0, translate(err, "count filtered users")
	}
	return n, nil
}

// CountActiveByRole reports how many active (status = 'active') users hold a
// role. Unlike CountByRole, it is part of user.Repository: Service.Update and
// Service.Delete depend on it to protect the last active system admin.
//
// This is a plain read with no concurrency guarantee — see LockActiveByRole
// for the version the actual guard runs inside a transaction.
func (r *Repository) CountActiveByRole(ctx context.Context, role auth.Role) (int64, error) {
	n, err := r.q(ctx).CountActiveUsersByRole(ctx, string(role))
	if err != nil {
		return 0, translate(err, "count active users by role")
	}
	return n, nil
}

// LockActiveByRole locks every active row holding role, for the duration of
// the caller's transaction, and returns how many there are. It must be
// called from inside that transaction — see queries/user.sql's
// LockActiveUsersByRole for why the lock, not just the count, is what makes
// the last-system-admin guard correct under concurrency.
func (r *Repository) LockActiveByRole(ctx context.Context, role auth.Role) (int64, error) {
	rows, err := r.q(ctx).LockActiveUsersByRole(ctx, string(role))
	if err != nil {
		return 0, translate(err, "lock active users by role")
	}
	return int64(len(rows)), nil
}

// ActiveUserIDsForRoles returns the active users holding any of roles, with
// no duplicates.
//
// Like CountByRole, this is deliberately not part of user.Repository: it
// exists for package incident's notification fan-out (see
// incident.RecipientLookup), a caller with no principal in context and so no
// use for user.Service's own permission-gated listing.
func (r *Repository) ActiveUserIDsForRoles(ctx context.Context, roles []string) ([]string, error) {
	seen := make(map[string]bool)
	ids := make([]string, 0, len(roles))
	for _, role := range roles {
		rows, err := r.q(ctx).ListActiveUserIDsByRole(ctx, role)
		if err != nil {
			return nil, translate(err, "list active users by role")
		}
		for _, id := range rows {
			s := id.String()
			if seen[s] {
				continue
			}
			seen[s] = true
			ids = append(ids, s)
		}
	}
	return ids, nil
}

func roleFilter(r *auth.Role) *string {
	if r == nil {
		return nil
	}
	s := r.String()
	return &s
}

func statusFilter(st *user.Status) *string {
	if st == nil {
		return nil
	}
	s := st.String()
	return &s
}

// CountByRole reports how many users hold a role.
//
// It is deliberately not part of user.Repository: the service has no use for
// it, and widening that interface would oblige every implementation to support
// a query that exists for one command. The bootstrap command depends on this
// concrete type, so it can simply call it.
func (r *Repository) CountByRole(ctx context.Context, role auth.Role) (int64, error) {
	n, err := r.q(ctx).CountUsersByRole(ctx, string(role))
	if err != nil {
		return 0, translate(err, "count users by role")
	}
	return n, nil
}

// Update applies a partial update and returns the stored row.
func (r *Repository) Update(ctx context.Context, id user.ID, p user.Patch) (user.User, error) {
	params := db.UpdateUserParams{
		ID: id.UUID(), Email: p.Email, Username: p.Username,
		Name: p.Name, PasswordHash: p.PasswordHash,
	}
	if p.Status != nil {
		s := p.Status.String()
		params.Status = &s
	}
	if p.Role != nil {
		role := p.Role.String()
		params.Role = &role
	}
	departmentID, err := parseOptionalUUID(p.DepartmentID)
	if err != nil {
		return user.User{}, apperr.Invalid("INVALID_ARGUMENT", "department_id must be a UUID",
			apperr.Detail{Field: "department_id", Reason: "not a UUID"})
	}
	params.DepartmentID = departmentID
	params.ClearDepartment = p.ClearDepartment

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
	u := user.User{
		ID:           user.ID(row.ID),
		Email:        row.Email,
		Username:     derefString(row.Username),
		PasswordHash: row.PasswordHash,
		Name:         row.Name,
		Status:       user.Status(row.Status),
		Role:         auth.Role(row.Role),
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
	if row.DepartmentID != nil {
		s := row.DepartmentID.String()
		u.DepartmentID = &s
	}
	return u
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
			switch pgErr.ConstraintName {
			case "users_email_key":
				return apperr.Wrap(err, apperr.KindConflict, "EMAIL_ALREADY_EXISTS",
					"a user with this email already exists")
			case "users_username_key":
				return apperr.Wrap(err, apperr.KindConflict, "USERNAME_ALREADY_EXISTS",
					"this username is already taken")
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
