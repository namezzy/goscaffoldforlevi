package org_test

import (
	"context"
	"strings"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/org"
)

// fakeRepository stores departments the way a real uuid column would: every
// id that comes in through CreateDepartment or UpdateDepartment's own id
// parameter is trusted to already be canonical (org.Service normalizes
// before calling either), but ParentID values are stored verbatim on the
// struct exactly as the service handed them over — which is what lets
// TestRejectCycleCatchesADifferentlyFormattedUUID below prove the service,
// not the repository, is what has to normalize.
type fakeRepository struct {
	byID map[string]org.Department
}

func newFakeRepository() *fakeRepository { return &fakeRepository{byID: map[string]org.Department{}} }

func (f *fakeRepository) GetOrganization(context.Context) (org.Organization, error) {
	return org.Organization{Name: "Test Org"}, nil
}

func (f *fakeRepository) UpdateOrganization(_ context.Context, p org.OrganizationPatch) (org.Organization, error) {
	name := "Test Org"
	if p.Name != nil {
		name = *p.Name
	}
	return org.Organization{Name: name}, nil
}

func (f *fakeRepository) CreateDepartment(_ context.Context, d org.Department) (org.Department, error) {
	f.byID[d.ID] = d
	return d, nil
}

func (f *fakeRepository) GetDepartment(_ context.Context, id string) (org.Department, error) {
	d, ok := f.byID[id]
	if !ok {
		return org.Department{}, apperr.NotFound("DEPARTMENT_NOT_FOUND", "department not found")
	}
	return d, nil
}

func (f *fakeRepository) ListDepartments(context.Context) ([]org.Department, error) {
	out := make([]org.Department, 0, len(f.byID))
	for _, d := range f.byID {
		out = append(out, d)
	}
	return out, nil
}

// LockDepartmentsForReparenting has no lock to take in-process — this fake
// is single-goroutine, so it is ListDepartments under another name. The
// real guarantee is proved in internal/org/orgpg's integration test against
// real PostgreSQL, the same split TestConcurrentDemotionsCannotBothSucceed
// (internal/user/userpg) uses for the analogous last-system-admin race.
func (f *fakeRepository) LockDepartmentsForReparenting(ctx context.Context) ([]org.Department, error) {
	return f.ListDepartments(ctx)
}

func (f *fakeRepository) UpdateDepartment(_ context.Context, id string, p org.DepartmentPatch) (org.Department, error) {
	d, ok := f.byID[id]
	if !ok {
		return org.Department{}, apperr.NotFound("DEPARTMENT_NOT_FOUND", "department not found")
	}
	if p.Name != nil {
		d.Name = *p.Name
	}
	if p.ClearParent {
		d.ParentID = nil
	} else if p.ParentID != nil {
		d.ParentID = p.ParentID
	}
	f.byID[id] = d
	return d, nil
}

func (f *fakeRepository) DeleteDepartment(_ context.Context, id string) error {
	if _, ok := f.byID[id]; !ok {
		return apperr.NotFound("DEPARTMENT_NOT_FOUND", "department not found")
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeRepository) CountChildDepartments(_ context.Context, id string) (int64, error) {
	var n int64
	for _, d := range f.byID {
		if d.ParentID != nil && *d.ParentID == id {
			n++
		}
	}
	return n, nil
}

func (f *fakeRepository) CountDepartmentMembers(context.Context, string) (int64, error) {
	return 0, nil
}

type directTx struct{}

func (directTx) WithTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type nopAuditRecorder struct{}

func (nopAuditRecorder) Record(context.Context, audit.Entry) error { return nil }

func adminContext() context.Context {
	return auth.ContextWithPrincipal(context.Background(),
		auth.Principal{UserID: "admin-1", Role: auth.RoleSystemAdmin})
}

func newService(repo org.Repository) *org.Service {
	return org.NewService(repo, directTx{}, nopAuditRecorder{}, nil)
}

// TestRejectCycleCatchesADifferentlyFormattedUUID is the regression test for
// the bypass a code review found: the cycle check keys a map by the
// canonical UUID form ListDepartments returns, and used to compare it
// against whatever spelling the caller's parent_id arrived in — upper case,
// no hyphens, all equally valid UUID text, all equally invisible to a map
// lookup keyed by a different spelling of the same value. A becomes B's
// parent; this asks to move B's parent to A, but spelled without hyphens
// and upper case.
func TestRejectCycleCatchesADifferentlyFormattedUUID(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := newService(repo)

	a, err := svc.CreateDepartment(adminContext(), org.NewDepartmentInput{Name: "A"})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	b, err := svc.CreateDepartment(adminContext(), org.NewDepartmentInput{Name: "B", ParentID: &a.ID})
	if err != nil {
		t.Fatalf("create B: %v", err)
	}

	mangled := strings.ToUpper(strings.ReplaceAll(b.ID, "-", ""))
	// Sanity check on the test's own premise: a different spelling of the
	// same UUID, not a typo of some other value.
	if mangled == b.ID {
		t.Fatalf("test fixture bug: mangled id %q is identical to the original", mangled)
	}

	_, err = svc.UpdateDepartment(adminContext(), a.ID, org.DepartmentPatch{ParentID: &mangled})
	if !apperr.Is(err, apperr.KindInvalidArgument) {
		t.Fatalf("UpdateDepartment() kind = %v, want invalid argument (cycle detected)", apperr.KindOf(err))
	}
}

// TestUpdateDepartmentRejectsAMalformedParentID is the direct companion:
// something that is not a UUID at all, in any spelling, must be rejected
// with a clear validation error rather than silently failing the cycle
// check's map lookup (which would report "no cycle" for any string that
// matches nothing, malformed values included) and reaching the database.
func TestUpdateDepartmentRejectsAMalformedParentID(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := newService(repo)
	a, err := svc.CreateDepartment(adminContext(), org.NewDepartmentInput{Name: "A"})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}

	notAUUID := "not-a-uuid"
	_, err = svc.UpdateDepartment(adminContext(), a.ID, org.DepartmentPatch{ParentID: &notAUUID})
	if !apperr.Is(err, apperr.KindInvalidArgument) {
		t.Fatalf("UpdateDepartment() kind = %v, want invalid argument", apperr.KindOf(err))
	}
}

// TestUpdateDepartmentAllowsAnOrdinaryReparent is the non-cycle baseline:
// the normalization and locked cycle check must not reject a perfectly
// ordinary move.
func TestUpdateDepartmentAllowsAnOrdinaryReparent(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := newService(repo)
	a, err := svc.CreateDepartment(adminContext(), org.NewDepartmentInput{Name: "A"})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	b, err := svc.CreateDepartment(adminContext(), org.NewDepartmentInput{Name: "B"})
	if err != nil {
		t.Fatalf("create B: %v", err)
	}

	updated, err := svc.UpdateDepartment(adminContext(), b.ID, org.DepartmentPatch{ParentID: &a.ID})
	if err != nil {
		t.Fatalf("UpdateDepartment() = %v, want nil", err)
	}
	if updated.ParentID == nil || *updated.ParentID != a.ID {
		t.Errorf("ParentID = %v, want %q", updated.ParentID, a.ID)
	}
}
