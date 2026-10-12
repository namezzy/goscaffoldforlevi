//go:build integration

package userpg_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/pgtest"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
	"github.com/namezzy/goscaffoldforlevi/internal/user/userpg"
)

type stubHasher struct{}

func (stubHasher) Hash(string) (string, error) { return "$stub$hash", nil }

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// asSystemAdmin builds a context whose principal is userID, holding
// system_admin — what every caller in this test acts as, since the point is
// two system admins each demoting the other, not an authorization check.
func asSystemAdmin(userID string) context.Context {
	return auth.ContextWithPrincipal(context.Background(),
		auth.Principal{UserID: userID, Role: auth.RoleSystemAdmin})
}

// TestConcurrentDemotionsCannotBothSucceed is the regression test for the
// race a code review found in guardLastSystemAdmin's original
// implementation: it counted active system admins in a plain SELECT taken
// before the write's own transaction opened, so two concurrent requests
// each demoting one of the last two admins could both read "2 active"
// before either wrote, and both would then proceed — leaving the
// organization with zero.
//
// This exercises the real *userpg.Repository and a real PostgreSQL
// transaction manager (postgres.NewTxManager), not the fake repository the
// rest of this package's tests use: the fix depends on a genuine row lock
// (SELECT ... FOR UPDATE) serializing two real, concurrently-committing
// transactions, which nothing in-process can simulate.
func TestConcurrentDemotionsCannotBothSucceed(t *testing.T) {
	pg := pgtest.Start(t)
	ctx := context.Background()

	repo := userpg.New(pg.Pool)
	tx := postgres.NewTxManager(pg.Pool)
	svc := user.NewService(repo, stubHasher{}, tx, messaging.NewNoopPublisher(discardLogger()), discardLogger())

	first, err := repo.Create(ctx, user.User{
		ID: user.NewID(), Email: "first-admin@example.com", Name: "First Admin",
		Status: user.StatusActive, Role: auth.RoleSystemAdmin, PasswordHash: "$stub$hash",
	})
	if err != nil {
		t.Fatalf("seed first admin: %v", err)
	}
	second, err := repo.Create(ctx, user.User{
		ID: user.NewID(), Email: "second-admin@example.com", Name: "Second Admin",
		Status: user.StatusActive, Role: auth.RoleSystemAdmin, PasswordHash: "$stub$hash",
	})
	if err != nil {
		t.Fatalf("seed second admin: %v", err)
	}

	readOnly := auth.RoleReadOnly
	results := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, results[0] = svc.Update(asSystemAdmin(first.ID.String()), second.ID, user.Patch{Role: &readOnly})
	}()
	go func() {
		defer wg.Done()
		_, results[1] = svc.Update(asSystemAdmin(second.ID.String()), first.ID, user.Patch{Role: &readOnly})
	}()
	wg.Wait()

	succeeded, refused := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			succeeded++
		case apperr.Is(err, apperr.KindConflict):
			refused++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if succeeded != 1 || refused != 1 {
		t.Fatalf("succeeded=%d refused=%d, want exactly one of each — the race left an inconsistent count", succeeded, refused)
	}

	remaining, err := repo.CountActiveByRole(ctx, auth.RoleSystemAdmin)
	if err != nil {
		t.Fatalf("count active system admins: %v", err)
	}
	if remaining != 1 {
		t.Errorf("active system admins remaining = %d, want exactly 1 — the organization must never be left with zero", remaining)
	}
}
