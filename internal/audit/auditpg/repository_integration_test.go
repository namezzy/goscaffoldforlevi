//go:build integration

package auditpg_test

import (
	"context"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/audit/auditpg"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/pgtest"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
	"github.com/namezzy/goscaffoldforlevi/internal/user/userpg"
)

// TestDeletingAPastActorDoesNotFailOnTheAuditTrigger is the regression test
// for the bug a code review found: audit_events.actor_id originally
// referenced users(id) ON DELETE SET NULL, and audit_events' own trigger
// (org_roles_audit migration) refuses UPDATE unconditionally to keep the
// log append-only. Deleting a user who had ever been the actor of a past
// audit event tripped both rules at once — PostgreSQL tried to null out the
// reference and the trigger refused — so the delete failed outright with
// "audit_events is append-only: UPDATE is not permitted", for a user with
// no connection to the delete beyond having once changed their own
// password. The fix (see the audit_actor_id_no_fk migration) drops the
// foreign key; actor_id keeps its value by convention, not by constraint.
func TestDeletingAPastActorDoesNotFailOnTheAuditTrigger(t *testing.T) {
	pg := pgtest.Start(t)
	ctx := context.Background()

	users := userpg.New(pg.Pool)
	audits := auditpg.New(pg.Pool)

	actor, err := users.Create(ctx, user.User{
		ID: user.NewID(), Email: "past-actor@example.com", Name: "Past Actor",
		Status: user.StatusActive, Role: auth.RoleReadOnly, PasswordHash: "$stub$hash",
	})
	if err != nil {
		t.Fatalf("create actor: %v", err)
	}
	actorID := actor.ID.String()

	if err := audits.Insert(ctx, audit.Record{
		ID: user.NewID().String(), ActorID: &actorID, Action: "member.password_changed",
		TargetType: "user", TargetID: actorID, Result: audit.ResultSuccess,
	}); err != nil {
		t.Fatalf("insert audit record: %v", err)
	}

	// This is the assertion: the delete must succeed even though a past
	// audit record's actor_id points at this exact user.
	if err := users.Delete(ctx, actor.ID); err != nil {
		t.Fatalf("Delete() = %v, want nil — a past audit actor must not block account deletion", err)
	}

	// And the historical record must still be there afterwards, actor_id
	// value intact, which is the entire point of dropping the foreign key
	// rather than, say, deleting the audit rows along with the account.
	fetched, err := audits.Get(ctx, mustFindEventID(ctx, t, audits, actorID))
	if err != nil {
		t.Fatalf("Get() the audit record after the actor's deletion: %v", err)
	}
	if fetched.ActorID == nil || *fetched.ActorID != actorID {
		t.Errorf("ActorID = %v after deletion, want it preserved as %q", fetched.ActorID, actorID)
	}
}

// mustFindEventID is a small helper: Insert does not hand back the id it
// generated, so the test recovers it via List instead of hard-coding a
// second, redundant identifier.
func mustFindEventID(ctx context.Context, t *testing.T, audits *auditpg.Repository, actorID string) string {
	t.Helper()
	records, err := audits.List(ctx, audit.Filter{ActorID: &actorID}, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("List() returned %d records, want 1", len(records))
	}
	return records[0].ID
}
