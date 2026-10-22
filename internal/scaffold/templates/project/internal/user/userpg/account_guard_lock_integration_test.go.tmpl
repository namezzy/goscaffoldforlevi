//go:build integration

package userpg_test

import (
	"context"
	"testing"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/pgtest"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
	"github.com/namezzy/goscaffoldforlevi/internal/user/userpg"
)

// asMemberAdmin builds a context for a principal holding member_admin:
// user:update, but no authority over a system administrator's account.
func asMemberAdmin(userID string) context.Context {
	return auth.ContextWithPrincipal(context.Background(),
		auth.Principal{UserID: userID, Role: auth.RoleMemberAdmin})
}

// TestAGuardWaitsForAConcurrentPromotionToCommit is the regression test for
// the second half of the same race: the account-control guard used to read
// the target's role in a plain SELECT taken before the write's transaction
// opened, so a member_admin could disable an account that had become a
// system administrator in the meantime.
//
// It is deterministic rather than a two-goroutine gamble: a transaction
// opened by the test itself holds the target row locked while promoting it,
// so a correct Update must block on that lock and only then decide. The
// fake repository in the service's own tests cannot reproduce this — it has
// no lock to block on — which is why this lives here, against real
// PostgreSQL, like TestConcurrentDemotionsCannotBothSucceed.
func TestAGuardWaitsForAConcurrentPromotionToCommit(t *testing.T) {
	pg := pgtest.Start(t)
	ctx := context.Background()

	repo := userpg.New(pg.Pool)
	tx := postgres.NewTxManager(pg.Pool)
	svc := user.NewService(repo, stubHasher{}, tx, messaging.NewNoopPublisher(discardLogger()), discardLogger())

	target, err := repo.Create(ctx, user.User{
		ID: user.NewID(), Email: "target@example.com", Name: "Target",
		Status: user.StatusActive, Role: auth.RoleReadOnly, PasswordHash: "$stub$hash",
	})
	if err != nil {
		t.Fatalf("seed target: %v", err)
	}

	// The competing transaction: it promotes the target and keeps the row
	// locked until it commits, below.
	promoting, err := pg.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin promoting transaction: %v", err)
	}
	defer promoting.Rollback(ctx) //nolint:errcheck // committed below; this is the failure path
	if _, err := promoting.Exec(ctx,
		"UPDATE users SET role = 'system_admin' WHERE id = $1", target.ID.UUID()); err != nil {
		t.Fatalf("promote target: %v", err)
	}

	disabled := user.StatusDisabled
	result := make(chan error, 1)
	go func() {
		_, err := svc.Update(asMemberAdmin("member-admin-1"), target.ID, user.Patch{Status: &disabled})
		result <- err
	}()

	select {
	case err := <-result:
		t.Fatalf("Update() returned %v while the row was locked — it decided on a role it had not locked", err)
	case <-time.After(500 * time.Millisecond):
		// Blocked on the lock, which is the point.
	}

	if err := promoting.Commit(ctx); err != nil {
		t.Fatalf("commit promotion: %v", err)
	}

	select {
	case err := <-result:
		if !apperr.Is(err, apperr.KindForbidden) {
			t.Fatalf("Update() kind = %v, want forbidden", apperr.KindOf(err))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Update() never returned after the promotion committed")
	}

	after, err := repo.GetByID(ctx, target.ID)
	if err != nil {
		t.Fatalf("reload target: %v", err)
	}
	if after.Status != user.StatusActive {
		t.Errorf("status = %v, want active — the promoted administrator was disabled anyway", after.Status)
	}
	if after.Role != auth.RoleSystemAdmin {
		t.Errorf("role = %v, want system_admin — the promotion itself was lost", after.Role)
	}
}
