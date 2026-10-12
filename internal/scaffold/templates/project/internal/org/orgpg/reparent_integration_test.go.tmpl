//go:build integration

package orgpg_test

import (
	"context"
	"sync"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/org"
	"github.com/namezzy/goscaffoldforlevi/internal/org/orgpg"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/pgtest"
)

type nopAuditRecorder struct{}

func (nopAuditRecorder) Record(context.Context, audit.Entry) error { return nil }

func asSystemAdmin() context.Context {
	return auth.ContextWithPrincipal(context.Background(),
		auth.Principal{UserID: "admin-1", Role: auth.RoleSystemAdmin})
}

// TestConcurrentMutualReparentCannotCreateACycle is the regression test for
// the race a code review found on top of the UUID-formatting bypass
// (internal/org's TestRejectCycleCatchesADifferentlyFormattedUUID): even
// with every id normalized, two departments reparented at the same
// moment — A moving under B, B moving under A — could each have their
// cycle check run against a snapshot of the tree taken before either write
// landed. Both checks see the pre-move tree, both see no cycle, both
// commit, and the result is a real A<->B cycle neither check ever
// observed. This exercises the real *orgpg.Repository and a real
// postgres.NewTxManager, the same way
// internal/user/userpg's TestConcurrentDemotionsCannotBothSucceed does for
// the analogous last-system-admin race — nothing in-process can simulate
// two transactions genuinely serializing on a row lock.
func TestConcurrentMutualReparentCannotCreateACycle(t *testing.T) {
	pg := pgtest.Start(t)

	repo := orgpg.New(pg.Pool)
	tx := postgres.NewTxManager(pg.Pool)
	svc := org.NewService(repo, tx, nopAuditRecorder{}, nil)

	a, err := svc.CreateDepartment(asSystemAdmin(), org.NewDepartmentInput{Name: "A"})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	b, err := svc.CreateDepartment(asSystemAdmin(), org.NewDepartmentInput{Name: "B"})
	if err != nil {
		t.Fatalf("create B: %v", err)
	}

	results := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, results[0] = svc.UpdateDepartment(asSystemAdmin(), a.ID, org.DepartmentPatch{ParentID: &b.ID})
	}()
	go func() {
		defer wg.Done()
		_, results[1] = svc.UpdateDepartment(asSystemAdmin(), b.ID, org.DepartmentPatch{ParentID: &a.ID})
	}()
	wg.Wait()

	succeeded, refused := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			succeeded++
		case apperr.Is(err, apperr.KindInvalidArgument):
			refused++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if succeeded != 1 || refused != 1 {
		t.Fatalf("succeeded=%d refused=%d, want exactly one of each — a race let both moves through, or refused both", succeeded, refused)
	}

	// Whichever move won, the tree must have no cycle: walking up from
	// either department must eventually reach a department with no parent.
	all, err := repo.ListDepartments(context.Background())
	if err != nil {
		t.Fatalf("list departments: %v", err)
	}
	byID := make(map[string]org.Department, len(all))
	for _, d := range all {
		byID[d.ID] = d
	}
	visited := map[string]bool{}
	for cursor := a.ID; cursor != ""; {
		if visited[cursor] {
			t.Fatalf("department tree contains a cycle reachable from A (at %s)", cursor)
		}
		visited[cursor] = true
		d, ok := byID[cursor]
		if !ok || d.ParentID == nil {
			break
		}
		cursor = *d.ParentID
	}
}
