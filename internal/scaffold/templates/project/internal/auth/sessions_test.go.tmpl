package auth_test

import (
	"context"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
)

func TestListSessionsReturnsOnlyTheCallersOwnSessions(t *testing.T) {
	t.Parallel()
	mine := accountFor(t, "user-1", true)
	someoneElses := accountFor(t, "user-2", true)
	f := newServiceFixture(t, newFakeAccounts().with("mine@example.com", mine).with("theirs@example.com", someoneElses))

	if _, err := f.svc.Login(context.Background(), "mine@example.com", testPassword, auth.RequestMeta{}); err != nil {
		t.Fatalf("Login() as user-1 = %v", err)
	}
	if _, err := f.svc.Login(context.Background(), "theirs@example.com", testPassword, auth.RequestMeta{}); err != nil {
		t.Fatalf("Login() as user-2 = %v", err)
	}

	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{UserID: "user-1", Role: auth.RoleReadOnly})
	sessions, err := f.svc.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions() = %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("returned %d sessions, want 1 (only the caller's own)", len(sessions))
	}
	if sessions[0].UserID != "user-1" {
		t.Errorf("leaked another user's session: %+v", sessions[0])
	}
}

func TestListSessionsRequiresAuthentication(t *testing.T) {
	t.Parallel()
	f := newServiceFixture(t, newFakeAccounts())

	if _, err := f.svc.ListSessions(context.Background()); !apperr.Is(err, apperr.KindUnauthorized) {
		t.Errorf("err = %v, want UNAUTHENTICATED", err)
	}
}

func TestRevokeSessionRefusesASessionThatBelongsToSomeoneElse(t *testing.T) {
	t.Parallel()
	owner := accountFor(t, "user-1", true)
	attacker := accountFor(t, "user-2", true)
	f := newServiceFixture(t, newFakeAccounts().with("owner@example.com", owner).with("attacker@example.com", attacker))

	if _, err := f.svc.Login(context.Background(), "owner@example.com", testPassword, auth.RequestMeta{}); err != nil {
		t.Fatalf("Login() as owner = %v", err)
	}
	targetSessionID := f.sessions.rows[0].FamilyID

	attackerCtx := auth.ContextWithPrincipal(context.Background(), auth.Principal{UserID: "user-2", Role: auth.RoleReadOnly})
	err := f.svc.RevokeSession(attackerCtx, targetSessionID)
	if !apperr.Is(err, apperr.KindNotFound) {
		t.Errorf("revoking someone else's session: err = %v, want SESSION_NOT_FOUND (not a permission error, which would confirm the session exists)", err)
	}
	if f.sessions.rows[0].RevokedAt != nil {
		t.Error("the session was revoked despite belonging to a different user")
	}
}

func TestRevokeSessionRevokesTheCallersOwnSession(t *testing.T) {
	t.Parallel()
	account := accountFor(t, "user-1", true)
	f := newServiceFixture(t, newFakeAccounts().with("a@example.com", account))

	if _, err := f.svc.Login(context.Background(), "a@example.com", testPassword, auth.RequestMeta{}); err != nil {
		t.Fatalf("Login() = %v", err)
	}
	id := f.sessions.rows[0].FamilyID

	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{UserID: "user-1", Role: auth.RoleReadOnly})
	if err := f.svc.RevokeSession(ctx, id); err != nil {
		t.Fatalf("RevokeSession() = %v", err)
	}
	if f.sessions.rows[0].RevokedAt == nil {
		t.Error("the session was not revoked")
	}
}

func TestRevokeSessionOnAnAlreadyRevokedSessionIsNotFound(t *testing.T) {
	t.Parallel()
	account := accountFor(t, "user-1", true)
	f := newServiceFixture(t, newFakeAccounts().with("a@example.com", account))

	if _, err := f.svc.Login(context.Background(), "a@example.com", testPassword, auth.RequestMeta{}); err != nil {
		t.Fatalf("Login() = %v", err)
	}
	id := f.sessions.rows[0].FamilyID
	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{UserID: "user-1", Role: auth.RoleReadOnly})

	if err := f.svc.RevokeSession(ctx, id); err != nil {
		t.Fatalf("first RevokeSession() = %v", err)
	}
	if err := f.svc.RevokeSession(ctx, id); !apperr.Is(err, apperr.KindNotFound) {
		t.Errorf("second RevokeSession() = %v, want SESSION_NOT_FOUND", err)
	}
}

func TestRevokeOtherSessionsLeavesTheCurrentSessionActive(t *testing.T) {
	t.Parallel()
	account := accountFor(t, "user-1", true)
	f := newServiceFixture(t, newFakeAccounts().with("a@example.com", account))

	first, err := f.svc.Login(context.Background(), "a@example.com", testPassword, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("first Login() = %v", err)
	}
	if _, err := f.svc.Login(context.Background(), "a@example.com", testPassword, auth.RequestMeta{}); err != nil {
		t.Fatalf("second Login() = %v", err)
	}
	if _, err := f.svc.Login(context.Background(), "a@example.com", testPassword, auth.RequestMeta{}); err != nil {
		t.Fatalf("third Login() = %v", err)
	}
	if f.sessions.activeCount() != 3 {
		t.Fatalf("active sessions = %d, want 3", f.sessions.activeCount())
	}

	// The context's TokenID is the family id embedded as the access
	// token's jti — see issuePair's doc comment — which is what
	// RevokeOtherSessions parses back out to know which session to spare.
	ctx := auth.ContextWithPrincipal(context.Background(),
		auth.Principal{UserID: "user-1", Role: auth.RoleReadOnly, TokenID: first.SessionID})

	revoked, err := f.svc.RevokeOtherSessions(ctx)
	if err != nil {
		t.Fatalf("RevokeOtherSessions() = %v", err)
	}
	if revoked != 2 {
		t.Errorf("revoked = %d, want 2", revoked)
	}
	if f.sessions.activeCount() != 1 {
		t.Fatalf("active sessions after = %d, want 1", f.sessions.activeCount())
	}
	for _, row := range f.sessions.rows {
		if row.RevokedAt == nil && row.FamilyID.String() != first.SessionID {
			t.Errorf("session %s from a different family was left active", row.ID)
		}
	}
}

func TestRevokeOtherSessionsRequiresAuthentication(t *testing.T) {
	t.Parallel()
	f := newServiceFixture(t, newFakeAccounts())

	if _, err := f.svc.RevokeOtherSessions(context.Background()); !apperr.Is(err, apperr.KindUnauthorized) {
		t.Errorf("err = %v, want UNAUTHENTICATED", err)
	}
}
