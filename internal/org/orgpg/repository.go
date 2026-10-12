// Package orgpg is the PostgreSQL implementation of the org repository.
package orgpg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/org"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/db"
)

const (
	codeForeignKeyViolation = "23503"
	codeCheckViolation      = "23514"
)

// Repository stores the organization and its departments.
type Repository struct {
	pool *pgxpool.Pool
}

// New returns a Repository backed by pool.
func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) q(ctx context.Context) *db.Queries {
	return db.New(postgres.Querier(ctx, r.pool))
}

// GetOrganization reads the singleton organization row.
func (r *Repository) GetOrganization(ctx context.Context) (org.Organization, error) {
	row, err := r.q(ctx).GetOrganization(ctx)
	if err != nil {
		return org.Organization{}, translate(err, "get organization")
	}
	return toOrganizationDomain(row), nil
}

// UpdateOrganization applies a partial update to the singleton row.
func (r *Repository) UpdateOrganization(ctx context.Context, p org.OrganizationPatch) (org.Organization, error) {
	row, err := r.q(ctx).UpdateOrganization(ctx, db.UpdateOrganizationParams{Name: p.Name, Identifier: p.Identifier})
	if err != nil {
		return org.Organization{}, translate(err, "update organization")
	}
	return toOrganizationDomain(row), nil
}

func toOrganizationDomain(row db.Organization) org.Organization {
	return org.Organization{
		Name: row.Name, Identifier: row.Identifier, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

// CreateDepartment inserts a department.
func (r *Repository) CreateDepartment(ctx context.Context, d org.Department) (org.Department, error) {
	id, err := uuid.Parse(d.ID)
	if err != nil {
		return org.Department{}, apperr.Internal(fmt.Errorf("department id: %w", err))
	}
	parentID, err := parseOptionalUUID(d.ParentID)
	if err != nil {
		return org.Department{}, apperr.Invalid("INVALID_ARGUMENT", "parent_id must be a UUID",
			apperr.Detail{Field: "parent_id", Reason: "not a UUID"})
	}
	row, err := r.q(ctx).CreateDepartment(ctx, db.CreateDepartmentParams{ID: id, Name: d.Name, ParentID: parentID})
	if err != nil {
		return org.Department{}, translate(err, "create department")
	}
	return toDomain(row), nil
}

// GetDepartment loads a department by ID.
func (r *Repository) GetDepartment(ctx context.Context, id string) (org.Department, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return org.Department{}, errDepartmentNotFound()
	}
	row, err := r.q(ctx).GetDepartmentByID(ctx, parsed)
	if err != nil {
		return org.Department{}, translate(err, "get department")
	}
	return toDomain(row), nil
}

// ListDepartments returns every department.
func (r *Repository) ListDepartments(ctx context.Context) ([]org.Department, error) {
	rows, err := r.q(ctx).ListDepartments(ctx)
	if err != nil {
		return nil, translate(err, "list departments")
	}
	out := make([]org.Department, len(rows))
	for i, row := range rows {
		out[i] = toDomain(row)
	}
	return out, nil
}

// LockDepartmentsForReparenting is ListDepartments' transactional sibling —
// see org.Repository's doc comment on this method for why the lock, not
// just the read, is what the cycle check actually depends on.
func (r *Repository) LockDepartmentsForReparenting(ctx context.Context) ([]org.Department, error) {
	rows, err := r.q(ctx).LockDepartmentsForReparenting(ctx)
	if err != nil {
		return nil, translate(err, "lock departments for reparenting")
	}
	out := make([]org.Department, len(rows))
	for i, row := range rows {
		out[i] = toDomain(row)
	}
	return out, nil
}

// UpdateDepartment applies a partial update, including reparenting/clearing.
func (r *Repository) UpdateDepartment(ctx context.Context, id string, p org.DepartmentPatch) (org.Department, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return org.Department{}, errDepartmentNotFound()
	}
	parentID, err := parseOptionalUUID(p.ParentID)
	if err != nil {
		return org.Department{}, apperr.Invalid("INVALID_ARGUMENT", "parent_id must be a UUID",
			apperr.Detail{Field: "parent_id", Reason: "not a UUID"})
	}
	row, err := r.q(ctx).UpdateDepartment(ctx, db.UpdateDepartmentParams{
		ID: parsed, Name: p.Name, ParentID: parentID, ClearParent: p.ClearParent,
	})
	if err != nil {
		return org.Department{}, translate(err, "update department")
	}
	return toDomain(row), nil
}

// DeleteDepartment removes a department.
func (r *Repository) DeleteDepartment(ctx context.Context, id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return errDepartmentNotFound()
	}
	n, err := r.q(ctx).DeleteDepartment(ctx, parsed)
	if err != nil {
		return translate(err, "delete department")
	}
	if n == 0 {
		return errDepartmentNotFound()
	}
	return nil
}

// CountChildDepartments reports how many departments have id as their parent.
func (r *Repository) CountChildDepartments(ctx context.Context, id string) (int64, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return 0, errDepartmentNotFound()
	}
	n, err := r.q(ctx).CountChildDepartments(ctx, &parsed)
	if err != nil {
		return 0, translate(err, "count child departments")
	}
	return n, nil
}

// CountDepartmentMembers reports how many users are assigned to department id.
func (r *Repository) CountDepartmentMembers(ctx context.Context, id string) (int64, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return 0, errDepartmentNotFound()
	}
	n, err := r.q(ctx).CountDepartmentMembers(ctx, &parsed)
	if err != nil {
		return 0, translate(err, "count department members")
	}
	return n, nil
}

func toDomain(row db.Department) org.Department {
	d := org.Department{ID: row.ID.String(), Name: row.Name, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	if row.ParentID != nil {
		s := row.ParentID.String()
		d.ParentID = &s
	}
	return d
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

func errDepartmentNotFound() error {
	return apperr.NotFound("DEPARTMENT_NOT_FOUND", "department not found")
}

func translate(err error, op string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errDepartmentNotFound()
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
