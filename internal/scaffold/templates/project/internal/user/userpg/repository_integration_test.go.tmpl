//go:build integration

package userpg_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/pgtest"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
	"github.com/namezzy/goscaffoldforlevi/internal/user/userpg"
)

// These tests exist because the repository's job is to translate PostgreSQL's
// behaviour, and PostgreSQL's behaviour is exactly what a fake would have to
// invent. A unique-violation test against an in-memory map proves that the map
// rejects duplicates.

func newUser(email string) user.User {
	return user.User{
		ID:           user.NewID(),
		Email:        email,
		Name:         "Test User",
		Status:       user.StatusActive,
		Role:         auth.RoleUser,
		PasswordHash: "$argon2id$v=19$m=65536,t=3,p=4$c2FsdHNhbHRzYWx0$aGFzaGhhc2hoYXNoaGFzaA",
	}
}

func kindOf(t *testing.T, err error) apperr.Kind {
	t.Helper()

	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error is not an *apperr.Error: %v", err)
	}
	return appErr.Kind
}

func TestCreateAndReadBackPreservesEveryField(t *testing.T) {
	pg := pgtest.Start(t)
	repo := userpg.New(pg.Pool)
	ctx := context.Background()

	want := newUser("roundtrip@example.com")
	created, err := repo.Create(ctx, want)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// The timestamps are assigned by the database, so the returned row is the
	// only place they can be observed.
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Error("the database did not populate the timestamps")
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Email != want.Email || got.Name != want.Name ||
		got.Role != want.Role || got.Status != want.Status ||
		got.PasswordHash != want.PasswordHash {
		t.Errorf("read back %+v, want the stored values", got)
	}

	byEmail, err := repo.GetByEmail(ctx, want.Email)
	if err != nil {
		t.Fatalf("get by email: %v", err)
	}
	if byEmail.ID != created.ID {
		t.Errorf("lookup by email returned %v, want %v", byEmail.ID, created.ID)
	}
}

// TestDuplicateEmailBecomesAConflict is the reason the service does not
// pre-check uniqueness with a SELECT: the index decides, and its violation has
// to arrive as a 409 rather than a 500.
func TestDuplicateEmailBecomesAConflict(t *testing.T) {
	pg := pgtest.Start(t)
	repo := userpg.New(pg.Pool)
	ctx := context.Background()

	first := newUser("dup@example.com")
	if _, err := repo.Create(ctx, first); err != nil {
		t.Fatalf("create first: %v", err)
	}

	_, err := repo.Create(ctx, newUser("dup@example.com"))
	if err == nil {
		t.Fatal("the second insert succeeded; the unique index is missing")
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error is not an *apperr.Error: %v", err)
	}
	if appErr.Kind != apperr.KindConflict {
		t.Errorf("kind = %v, want Conflict", appErr.Kind)
	}
	// Message is what reaches the client, so it must not carry the constraint
	// name or any other schema detail. The driver's text is kept as the wrapped
	// cause, where it is useful in a log and invisible to the caller.
	if strings.Contains(appErr.Message, "users_email_key") ||
		strings.Contains(appErr.Message, "SQLSTATE") {
		t.Errorf("the client-facing message leaks schema detail: %q", appErr.Message)
	}
	if !strings.Contains(err.Error(), "users_email_key") {
		t.Errorf("the driver detail was dropped instead of being wrapped: %v", err)
	}
}

func TestMissingRowsBecomeNotFound(t *testing.T) {
	pg := pgtest.Start(t)
	repo := userpg.New(pg.Pool)
	ctx := context.Background()
	absent := user.NewID()

	_, err := repo.GetByID(ctx, absent)
	if got := kindOf(t, err); got != apperr.KindNotFound {
		t.Errorf("GetByID kind = %v, want NotFound", got)
	}

	_, err = repo.GetByEmail(ctx, "nobody@example.com")
	if got := kindOf(t, err); got != apperr.KindNotFound {
		t.Errorf("GetByEmail kind = %v, want NotFound", got)
	}

	name := "New Name"
	_, err = repo.Update(ctx, absent, user.Patch{Name: &name})
	if got := kindOf(t, err); got != apperr.KindNotFound {
		t.Errorf("Update kind = %v, want NotFound", got)
	}

	// Delete must distinguish "removed a row" from "matched nothing"; without
	// checking the affected count it would answer 204 for any id.
	err = repo.Delete(ctx, absent)
	if got := kindOf(t, err); got != apperr.KindNotFound {
		t.Errorf("Delete kind = %v, want NotFound", got)
	}
}

// TestCheckConstraintViolationBecomesInvalidArgument covers the path where a
// value passes the service's validation but not the database's. The database is
// the last line of defence, and its refusal is the client's fault, not the
// server's.
func TestCheckConstraintViolationBecomesInvalidArgument(t *testing.T) {
	pg := pgtest.Start(t)
	repo := userpg.New(pg.Pool)
	ctx := context.Background()

	bad := newUser("badrole@example.com")
	bad.Role = auth.Role("wizard")

	_, err := repo.Create(ctx, bad)
	if err == nil {
		t.Fatal("the row was accepted; the CHECK constraint is missing")
	}
	if got := kindOf(t, err); got != apperr.KindInvalidArgument {
		t.Errorf("kind = %v, want InvalidArgument", got)
	}
}

// TestPagingIsStableWhenTimestampsCollide is the case the id tiebreaker exists
// for. Rows inserted inside the same transaction share created_at, so ordering
// by created_at alone leaves their relative order up to the planner — and a
// client paging through them sees duplicates and gaps.
func TestPagingIsStableWhenTimestampsCollide(t *testing.T) {
	pg := pgtest.Start(t)
	repo := userpg.New(pg.Pool)
	ctx := context.Background()

	const total = 9
	for i := 0; i < total; i++ {
		u := newUser(string(rune('a'+i)) + "@paging.example.com")
		if _, err := repo.Create(ctx, u); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}

	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != total {
		t.Fatalf("count = %d, want %d", count, total)
	}

	seen := map[user.ID]bool{}
	var previous user.User
	for offset := int32(0); offset < total; offset += 2 {
		page, err := repo.List(ctx, 2, offset)
		if err != nil {
			t.Fatalf("list at offset %d: %v", offset, err)
		}
		for _, u := range page {
			if seen[u.ID] {
				t.Errorf("user %v appeared on two pages", u.ID)
			}
			seen[u.ID] = true

			if !previous.CreatedAt.IsZero() {
				// Descending by created_at, then by id.
				if u.CreatedAt.After(previous.CreatedAt) ||
					(u.CreatedAt.Equal(previous.CreatedAt) && u.ID.String() > previous.ID.String()) {
					t.Errorf("ordering broke between %v and %v", previous.ID, u.ID)
				}
			}
			previous = u
		}
	}
	if len(seen) != total {
		t.Errorf("paged over %d distinct users, want %d", len(seen), total)
	}

	// An offset past the end is an empty page, not an error: a client asking
	// for page 100 of a short table has made a harmless mistake.
	page, err := repo.List(ctx, 2, total+10)
	if err != nil {
		t.Fatalf("list past the end: %v", err)
	}
	if len(page) != 0 {
		t.Errorf("page past the end returned %d rows", len(page))
	}
}

// TestUpdateTouchesOnlyTheNamedColumns checks the partial-update SQL. A patch
// that silently blanks the columns it does not mention is the classic failure
// of a hand-written UPDATE, and here it would erase a password hash.
func TestUpdateTouchesOnlyTheNamedColumns(t *testing.T) {
	pg := pgtest.Start(t)
	repo := userpg.New(pg.Pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newUser("patch@example.com"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	name := "Renamed"
	updated, err := repo.Update(ctx, created.ID, user.Patch{Name: &name})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Name != name {
		t.Errorf("name = %q, want %q", updated.Name, name)
	}
	if updated.PasswordHash != created.PasswordHash ||
		updated.Role != created.Role ||
		updated.Status != created.Status ||
		updated.Email != created.Email {
		t.Errorf("an unmentioned column changed: %+v", updated)
	}
	// The trigger, not the application, owns updated_at.
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Errorf("updated_at did not advance: %v then %v", created.UpdatedAt, updated.UpdatedAt)
	}
}

func TestDeleteRemovesTheRow(t *testing.T) {
	pg := pgtest.Start(t)
	repo := userpg.New(pg.Pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newUser("gone@example.com"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, created.ID); kindOf(t, err) != apperr.KindNotFound {
		t.Errorf("the row survived the delete")
	}

	// The email is free again, which is what makes deletion different from
	// disabling.
	reused := newUser("gone@example.com")
	if _, err := repo.Create(ctx, reused); err != nil {
		t.Errorf("the address stayed reserved after deletion: %v", err)
	}
}
