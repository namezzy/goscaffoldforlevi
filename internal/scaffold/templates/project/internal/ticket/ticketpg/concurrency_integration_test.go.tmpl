//go:build integration

package ticketpg_test

import (
	"context"
	"sync"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/notification"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/pgtest"
	"github.com/namezzy/goscaffoldforlevi/internal/ticket"
	"github.com/namezzy/goscaffoldforlevi/internal/ticket/ticketpg"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
	"github.com/namezzy/goscaffoldforlevi/internal/user/userpg"
)

type nopAuditRecorder struct{}

func (nopAuditRecorder) Record(context.Context, audit.Entry) error { return nil }

type nopActivityReader struct{}

func (nopActivityReader) List(context.Context, audit.Filter, int32, int32) ([]audit.Record, error) {
	return nil, nil
}

type nopNotifier struct{}

func (nopNotifier) Notify(context.Context, notification.NewNotification) error { return nil }

func asRequester(id string) context.Context {
	return auth.ContextWithPrincipal(context.Background(), auth.Principal{UserID: id, Role: auth.RoleOperator})
}

func asManager(id string) context.Context {
	return auth.ContextWithPrincipal(context.Background(), auth.Principal{UserID: id, Role: auth.RoleMemberAdmin})
}

// seedUser inserts a real user row via the userpg repository — a real
// UUIDv7 id, satisfying tickets.requester_id/assignee_id's foreign keys —
// and returns its id as a string. This mirrors the seeding pattern
// userpg's own integration tests use (see
// TestConcurrentDemotionsCannotBothSucceed) rather than hand-rolled SQL
// that would drift from the users table's own constraints.
func seedUser(t *testing.T, repo *userpg.Repository, email string) string {
	t.Helper()
	u, err := repo.Create(context.Background(), user.User{
		ID: user.NewID(), Email: email, Name: email, Status: user.StatusActive,
		Role: auth.RoleOperator, PasswordHash: "$stub$hash",
	})
	if err != nil {
		t.Fatalf("seed user %s: %v", email, err)
	}
	return u.ID.String()
}

// TestConcurrentProcessNoteCannotBothSucceed is the regression test for the
// optimistic-concurrency mechanism itself: "处理多人同时操作时的冲突，避免旧页面覆盖新状态"
// from the product spec. Two requests reading the same version and racing
// to write must not both land — exactly one commits and bumps the version,
// the other is refused with TICKET_VERSION_CONFLICT and must reload before
// retrying. This exercises the real *ticketpg.Repository and a real
// postgres.NewTxManager against actual concurrent transactions, the same
// way internal/org/orgpg's TestConcurrentMutualReparentCannotCreateACycle
// does for the analogous department race — nothing in-process (a fake
// repository's map) can reproduce two transactions genuinely racing on a
// row's current version.
func TestConcurrentProcessNoteCannotBothSucceed(t *testing.T) {
	pg := pgtest.Start(t)
	users := userpg.New(pg.Pool)
	requesterID := seedUser(t, users, "requester-1@example.com")
	assigneeID := seedUser(t, users, "assignee-1@example.com")

	repo := ticketpg.New(pg.Pool)
	tx := postgres.NewTxManager(pg.Pool)
	svc := ticket.NewService(repo, tx, nopAuditRecorder{}, nopActivityReader{}, nopNotifier{})

	created, err := svc.Create(asRequester(requesterID), ticket.NewTicketInput{
		Title: "printer jam", Description: "the printer jams on every page", Category: "facilities",
		Priority: ticket.PriorityNormal,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	assigned, err := svc.Assign(asManager("manager-1"), created.ID, assigneeID, created.Version)
	if err != nil {
		t.Fatalf("assign: %v", err)
	}

	results := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, results[0] = svc.ProcessNote(asRequester(assigneeID), assigned.ID, "checked the tray", assigned.Version)
	}()
	go func() {
		defer wg.Done()
		_, results[1] = svc.ProcessNote(asRequester(assigneeID), assigned.ID, "replaced the drum", assigned.Version)
	}()
	wg.Wait()

	succeeded, conflicted := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			succeeded++
		case apperr.Is(err, apperr.KindConflict):
			conflicted++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("succeeded=%d conflicted=%d, want exactly one of each — a race let both notes through, or refused both",
			succeeded, conflicted)
	}

	final, err := repo.GetByID(context.Background(), assigned.ID)
	if err != nil {
		t.Fatalf("get final ticket: %v", err)
	}
	if final.Version != assigned.Version+1 {
		t.Fatalf("final version = %d, want %d (exactly one write applied)", final.Version, assigned.Version+1)
	}
}
