// Package org holds the organization domain: the single company record and
// its department tree. It knows nothing about HTTP or SQL — those live in
// orghttp and orgpg respectively.
//
// This is a single-tenant design (Phase 1 work requirement): there is
// exactly one Organization row, enforced by the database itself (a boolean
// primary key constrained to true — see the org_roles_audit migration), not
// by a service-layer check that a second call could race past.
package org

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/validate"
)

// Organization is the single company record.
type Organization struct {
	Name string
	// Identifier is a short label for the organization — for example, a
	// slug or short code shown alongside its name in settings and in
	// system-generated content. It may be empty; nothing in this codebase
	// requires one.
	Identifier string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// OrganizationPatch describes a partial update, following the same
// pointer-means-PATCH-semantics convention as user.Patch.
type OrganizationPatch struct {
	Name       *string
	Identifier *string
}

// Department is one node in the org chart. ParentID is nil for a top-level
// department.
type Department struct {
	ID        string
	Name      string
	ParentID  *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DepartmentPatch describes a partial update to a department.
//
// ClearParent and ParentID are separate rather than "nil ParentID pointer
// means clear" for the same reason user.Patch needs one pointer per field:
// with a single *string, "move to root" and "leave the parent alone" would
// both be expressed as a nil pointer and become indistinguishable.
type DepartmentPatch struct {
	Name        *string
	ParentID    *string
	ClearParent bool
}

// NewDepartmentInput is the validated input for creating a department.
type NewDepartmentInput struct {
	Name     string
	ParentID *string
}

// Repository is the persistence port.
type Repository interface {
	GetOrganization(ctx context.Context) (Organization, error)
	UpdateOrganization(ctx context.Context, p OrganizationPatch) (Organization, error)

	CreateDepartment(ctx context.Context, d Department) (Department, error)
	GetDepartment(ctx context.Context, id string) (Department, error)
	ListDepartments(ctx context.Context) ([]Department, error)
	// LockDepartmentsForReparenting is ListDepartments' transactional
	// sibling: called from inside the write's own transaction, it locks
	// every department row so a concurrent reparent blocks until this one
	// commits — see queries/org.sql's LockDepartmentsForReparenting for why
	// that lock is what makes the cycle check correct when two departments
	// are reparented at the same time, not just when one is.
	LockDepartmentsForReparenting(ctx context.Context) ([]Department, error)
	UpdateDepartment(ctx context.Context, id string, p DepartmentPatch) (Department, error)
	DeleteDepartment(ctx context.Context, id string) error
	// CountChildDepartments and CountDepartmentMembers back DeleteDepartment's
	// guard: a department that still has either cannot be removed.
	CountChildDepartments(ctx context.Context, id string) (int64, error)
	CountDepartmentMembers(ctx context.Context, id string) (int64, error)
}

// TxManager runs a function inside a database transaction, so that a
// department write and its audit record commit atomically — see ADR-0005.
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// AuditRecorder records a management action. Declared here rather than
// depending on audit.Recorder directly, matching user.AuditRecorder's
// relationship to the audit package.
type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry) error
}

// NotificationChannel reports whether one delivery channel for
// system-triggered notifications (ticket assignment, invitations, and so
// on) is currently configured, and enough detail to say what "configured"
// means for that channel. It is read-only and deployment-controlled — see
// UpdateOrganization's split between what OrganizationPatch can change and
// what only the deployment environment can — so there is no corresponding
// write method here, only NotificationChannels.
type NotificationChannel struct {
	Name       string
	Configured bool
	Detail     string
}

// Service holds the org use cases.
type Service struct {
	repo                 Repository
	tx                   TxManager
	audit                AuditRecorder
	notificationChannels []NotificationChannel
}

// NewService wires the service. notificationChannels is the static,
// deployment-computed answer NotificationChannels reports — built once in
// the composition root from whatever senders are actually wired, not
// recomputed per request, since nothing about it can change without a
// redeploy.
func NewService(repo Repository, tx TxManager, auditRecorder AuditRecorder, notificationChannels []NotificationChannel) *Service {
	return &Service{repo: repo, tx: tx, audit: auditRecorder, notificationChannels: notificationChannels}
}

// NotificationChannels reports the current configuration status of every
// notification delivery channel this deployment knows about — the "通知渠道
// 状态" the settings screen shows. Gated by the same permission as reading
// the organization record itself: this is organization-level configuration
// visibility, not a distinct concern.
func (s *Service) NotificationChannels(ctx context.Context) ([]NotificationChannel, error) {
	if err := auth.Authorize(ctx, auth.PermOrgRead); err != nil {
		return nil, err
	}
	return s.notificationChannels, nil
}

// GetOrganization returns the company record.
func (s *Service) GetOrganization(ctx context.Context) (Organization, error) {
	if err := auth.Authorize(ctx, auth.PermOrgRead); err != nil {
		return Organization{}, err
	}
	return s.repo.GetOrganization(ctx)
}

// UpdateOrganization applies a partial update to the company record.
func (s *Service) UpdateOrganization(ctx context.Context, p OrganizationPatch) (Organization, error) {
	if err := auth.Authorize(ctx, auth.PermOrgUpdate); err != nil {
		return Organization{}, err
	}
	if p.Name == nil && p.Identifier == nil {
		return Organization{}, apperr.Invalid("EMPTY_PATCH", "the request changes nothing")
	}
	var v validate.Violations
	if p.Name != nil {
		v.Required("name", *p.Name)
		v.RuneLen("name", *p.Name, 1, 200)
		v.NoControlChars("name", *p.Name)
	}
	if p.Identifier != nil {
		v.RuneLen("identifier", *p.Identifier, 0, 100)
		v.NoControlChars("identifier", *p.Identifier)
	}
	if v.Any() {
		return Organization{}, v.Err("INVALID_ARGUMENT", "invalid organization")
	}

	before, err := s.repo.GetOrganization(ctx)
	if err != nil {
		return Organization{}, err
	}

	var updated Organization
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		updated, err = s.repo.UpdateOrganization(ctx, p)
		if err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{
			ActorID: actorID(ctx), Action: "organization.updated",
			TargetType: "organization", TargetID: "organization", Result: audit.ResultSuccess,
			Before: map[string]string{"name": before.Name, "identifier": before.Identifier},
			After:  map[string]string{"name": updated.Name, "identifier": updated.Identifier},
		})
	})
	if err != nil {
		return Organization{}, err
	}
	return updated, nil
}

// ListDepartments returns every department. The client builds the tree from
// this flat list — see queries/org.sql's ListDepartments for why this is not
// paginated.
func (s *Service) ListDepartments(ctx context.Context) ([]Department, error) {
	if err := auth.Authorize(ctx, auth.PermDepartmentRead); err != nil {
		return nil, err
	}
	return s.repo.ListDepartments(ctx)
}

// CreateDepartment adds a department to the tree.
func (s *Service) CreateDepartment(ctx context.Context, in NewDepartmentInput) (Department, error) {
	if err := auth.Authorize(ctx, auth.PermDepartmentWrite); err != nil {
		return Department{}, err
	}
	var v validate.Violations
	v.Required("name", in.Name)
	v.RuneLen("name", in.Name, 1, 100)
	v.NoControlChars("name", in.Name)
	if v.Any() {
		return Department{}, v.Err("INVALID_ARGUMENT", "invalid department")
	}
	// Normalized before it ever reaches a comparison — see
	// normalizeDepartmentID's doc comment for why an unnormalized UUID is
	// not just untidy but a real bypass elsewhere in this file.
	parentID, err := normalizeOptionalDepartmentID(in.ParentID)
	if err != nil {
		return Department{}, err
	}
	in.ParentID = parentID

	var created Department
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		created, err = s.repo.CreateDepartment(ctx, Department{
			ID: NewID(), Name: in.Name, ParentID: in.ParentID,
		})
		if err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{
			ActorID: actorID(ctx), Action: "department.created",
			TargetType: "department", TargetID: created.ID, Result: audit.ResultSuccess,
			After: map[string]any{"name": created.Name, "parent_id": created.ParentID},
		})
	})
	if err != nil {
		return Department{}, err
	}
	return created, nil
}

// UpdateDepartment applies a partial update, including reparenting.
func (s *Service) UpdateDepartment(ctx context.Context, id string, p DepartmentPatch) (Department, error) {
	if err := auth.Authorize(ctx, auth.PermDepartmentWrite); err != nil {
		return Department{}, err
	}
	if p.Name == nil && p.ParentID == nil && !p.ClearParent {
		return Department{}, apperr.Invalid("EMPTY_PATCH", "the request changes nothing")
	}
	var v validate.Violations
	if p.Name != nil {
		v.Required("name", *p.Name)
		v.RuneLen("name", *p.Name, 1, 100)
		v.NoControlChars("name", *p.Name)
	}
	if v.Any() {
		return Department{}, v.Err("INVALID_ARGUMENT", "invalid department")
	}

	// Both normalized to the same canonical form before any comparison —
	// see normalizeDepartmentID's doc comment. The cycle check below walks
	// a map keyed by the canonical form ListDepartments returns; an id or
	// parent_id that reached it in some other valid UUID spelling (upper
	// case, no hyphens) would silently fail to match, which is exactly how
	// the cycle check used to be bypassed.
	normalizedID, err := normalizeDepartmentID(id)
	if err != nil {
		return Department{}, err
	}
	id = normalizedID
	parentID, err := normalizeOptionalDepartmentID(p.ParentID)
	if err != nil {
		return Department{}, err
	}
	p.ParentID = parentID

	before, err := s.repo.GetDepartment(ctx, id)
	if err != nil {
		return Department{}, err
	}

	if p.ParentID != nil && *p.ParentID == id {
		return Department{}, apperr.Invalid("INVALID_ARGUMENT", "a department cannot be its own parent",
			apperr.Detail{Field: "parent_id", Reason: "equals the department's own id"})
	}

	var updated Department
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		// The cycle check runs in here, against a snapshot taken under
		// LockDepartmentsForReparenting's table-wide lock, not before the
		// transaction opens. Two concurrent reparents — A moving under B at
		// the same moment B moves under A — would each pass a check made
		// against the pre-move tree; this transaction's lock forces the
		// second one to wait for the first to commit, then re-derive the
		// tree including that commit before it gets to decide. See
		// rejectCycle and queries/org.sql for the rest of this reasoning.
		if p.ParentID != nil {
			if err := s.rejectCycle(ctx, id, *p.ParentID); err != nil {
				return err
			}
		}

		var err error
		updated, err = s.repo.UpdateDepartment(ctx, id, p)
		if err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{
			ActorID: actorID(ctx), Action: "department.updated",
			TargetType: "department", TargetID: id, Result: audit.ResultSuccess,
			Before: map[string]any{"name": before.Name, "parent_id": before.ParentID},
			After:  map[string]any{"name": updated.Name, "parent_id": updated.ParentID},
		})
	})
	if err != nil {
		return Department{}, err
	}
	return updated, nil
}

// rejectCycle walks up from newParentID toward the root and fails if it
// encounters id — the department being reparented. A CHECK constraint can
// only see the row it is validating, so a cycle longer than one hop (A's new
// parent is B, whose parent is already A) can only be caught here, by
// loading the tree and walking it.
//
// Both id and newParentID must already be normalized to the canonical UUID
// form ListDepartments' rows use (UpdateDepartment's two callers of this
// method both normalize before calling it) — byID is keyed by that form, so
// an unnormalized value (upper case, missing hyphens; all valid UUID text,
// all the same value to PostgreSQL) would silently fail to match a real
// entry and let a cycle through undetected.
func (s *Service) rejectCycle(ctx context.Context, id, newParentID string) error {
	// Must be called from inside the write's own transaction — see
	// LockDepartmentsForReparenting's doc comment. This locks and
	// re-reads the whole tree rather than trusting any snapshot taken
	// before the transaction opened.
	all, err := s.repo.LockDepartmentsForReparenting(ctx)
	if err != nil {
		return err
	}
	byID := make(map[string]Department, len(all))
	for _, d := range all {
		byID[d.ID] = d
	}
	// visited guards against an infinite loop if the stored tree somehow
	// already contains a cycle this check did not prevent — a defensive
	// backstop, not the primary mechanism: the primary mechanism is that
	// every cursor conducted through here already passed this same check
	// when it was assigned its own parent.
	visited := make(map[string]bool, len(all))
	for cursor := newParentID; cursor != ""; {
		if cursor == id {
			return apperr.Invalid("INVALID_ARGUMENT", "this would create a cycle in the department tree",
				apperr.Detail{Field: "parent_id", Reason: "descendant of the department being moved"})
		}
		if visited[cursor] {
			return apperr.Internal(fmt.Errorf("department tree already contains a cycle at %s", cursor))
		}
		visited[cursor] = true
		d, ok := byID[cursor]
		if !ok || d.ParentID == nil {
			break
		}
		cursor = *d.ParentID
	}
	return nil
}

// normalizeDepartmentID validates that s is a UUID and rewrites it to
// canonical form (lower case, hyphenated) — the form ListDepartments'
// results and every ID field on Department are already in, since they come
// back from a uuid column via orgpg's uuid.UUID.String(). Comparing an
// unnormalized value (valid UUID text, but not in that form) against those
// with == or a map lookup silently never matches, which is exactly how
// rejectCycle could previously be walked past: two UUID spellings of the
// same department compared unequal to each other.
func normalizeDepartmentID(s string) (string, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return "", apperr.Invalid("INVALID_ARGUMENT", "must be a UUID",
			apperr.Detail{Field: "id", Reason: "not a UUID"})
	}
	return id.String(), nil
}

// normalizeOptionalDepartmentID is normalizeDepartmentID for a *string that
// may be nil (parent_id, which is optional).
func normalizeOptionalDepartmentID(s *string) (*string, error) {
	if s == nil {
		return nil, nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil, apperr.Invalid("INVALID_ARGUMENT", "parent_id must be a UUID",
			apperr.Detail{Field: "parent_id", Reason: "not a UUID"})
	}
	normalized := id.String()
	return &normalized, nil
}

// DeleteDepartment removes a department. It refuses one that still has
// sub-departments or assigned members — see Repository's doc comment — so
// the caller gets a clear conflict instead of a raw foreign-key error.
func (s *Service) DeleteDepartment(ctx context.Context, id string) error {
	if err := auth.Authorize(ctx, auth.PermDepartmentWrite); err != nil {
		return err
	}
	before, err := s.repo.GetDepartment(ctx, id)
	if err != nil {
		return err
	}
	children, err := s.repo.CountChildDepartments(ctx, id)
	if err != nil {
		return err
	}
	if children > 0 {
		return apperr.Conflict("DEPARTMENT_HAS_CHILDREN", "move or remove its sub-departments first")
	}
	members, err := s.repo.CountDepartmentMembers(ctx, id)
	if err != nil {
		return err
	}
	if members > 0 {
		return apperr.Conflict("DEPARTMENT_HAS_MEMBERS", "reassign its members before deleting it")
	}

	return s.tx.WithTx(ctx, func(ctx context.Context) error {
		if err := s.repo.DeleteDepartment(ctx, id); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{
			ActorID: actorID(ctx), Action: "department.deleted",
			TargetType: "department", TargetID: id, Result: audit.ResultSuccess,
			Before: map[string]any{"name": before.Name, "parent_id": before.ParentID},
		})
	})
}

func actorID(ctx context.Context) *string {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil
	}
	id := p.UserID
	return &id
}
