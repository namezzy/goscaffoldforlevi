//go:build integration

package authpg_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/example/go-enterprise/internal/apperr"
	"github.com/example/go-enterprise/internal/auth"
	"github.com/example/go-enterprise/internal/auth/authpg"
	"github.com/example/go-enterprise/internal/postgres"
	"github.com/example/go-enterprise/internal/postgres/pgtest"
	"github.com/example/go-enterprise/internal/user"
	"github.com/example/go-enterprise/internal/user/userpg"
)

func kindOf(t *testing.T, err error) apperr.Kind {
	t.Helper()

	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error is not an *apperr.Error: %v", err)
	}
	return appErr.Kind
}

func seedUser(t *testing.T, pg *pgtest.Container, email string) user.User {
	t.Helper()

	u, err := userpg.New(pg.Pool).Create(context.Background(), user.User{
		ID:           user.NewID(),
		Email:        email,
		Name:         "Session Owner",
		Status:       user.StatusActive,
		Role:         auth.RoleUser,
		PasswordHash: "$argon2id$v=19$m=65536,t=3,p=4$c2FsdHNhbHRzYWx0$aGFzaGhhc2hoYXNoaGFzaA",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func hash(b byte) []byte {
	// The column requires exactly 32 bytes, matching a SHA-256 digest.
	out := make([]byte, 32)
	for i := range out {
		out[i] = b
	}
	return out
}

func newSession(u user.User, family auth.FamilyID, tokenHash []byte, parent *auth.SessionID) auth.NewSession {
	ip := netip.MustParseAddr("203.0.113.5")
	agent := "integration-test"
	return auth.NewSession{
		ID:        auth.SessionID(user.NewID().UUID()),
		UserID:    u.ID.String(),
		TokenHash: tokenHash,
		FamilyID:  family,
		ParentID:  parent,
		ExpiresAt: time.Now().Add(time.Hour),
		UserAgent: &agent,
		IP:        &ip,
	}
}

func TestSessionRoundTrip(t *testing.T) {
	pg := pgtest.Start(t)
	repo := authpg.NewSessionRepository(pg.Pool)
	ctx := context.Background()

	owner := seedUser(t, pg, "session@example.com")
	family := auth.FamilyID(user.NewID().UUID())

	created, err := repo.Create(ctx, newSession(owner, family, hash(1), nil))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.RevokedAt != nil {
		t.Error("a new session should not be revoked")
	}

	got, err := repo.GetByTokenHash(ctx, hash(1))
	if err != nil {
		t.Fatalf("get by token hash: %v", err)
	}
	if got.ID != created.ID || got.UserID != owner.ID.String() || got.FamilyID != family {
		t.Errorf("read back %+v, want the stored session", got)
	}
	if got.IP == nil || got.IP.String() != "203.0.113.5" {
		t.Errorf("ip = %v, want the recorded address", got.IP)
	}
	if !got.Active(time.Now()) {
		t.Error("a fresh session should be active")
	}
}

// TestOnlyTheDigestIsStored is the point of hashing the refresh token: a dump
// of this table must not yield anything a caller could present.
func TestOnlyTheDigestIsStored(t *testing.T) {
	pg := pgtest.Start(t)
	ctx := context.Background()
	owner := seedUser(t, pg, "digest@example.com")

	token, digest, err := auth.NewRefreshToken()
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	family := auth.FamilyID(user.NewID().UUID())
	if _, err := authpg.NewSessionRepository(pg.Pool).
		Create(ctx, newSession(owner, family, digest, nil)); err != nil {
		t.Fatalf("create: %v", err)
	}

	var found int
	if err := pg.Pool.QueryRow(ctx,
		`SELECT count(*) FROM refresh_sessions WHERE encode(token_hash, 'escape') = $1`,
		token,
	).Scan(&found); err != nil {
		t.Fatalf("query: %v", err)
	}
	if found != 0 {
		t.Error("the plaintext refresh token is present in the table")
	}
}

// TestTokenHashIsUnique prevents two live sessions from answering to one token.
func TestTokenHashIsUnique(t *testing.T) {
	pg := pgtest.Start(t)
	repo := authpg.NewSessionRepository(pg.Pool)
	ctx := context.Background()

	owner := seedUser(t, pg, "unique@example.com")
	family := auth.FamilyID(user.NewID().UUID())

	if _, err := repo.Create(ctx, newSession(owner, family, hash(2), nil)); err != nil {
		t.Fatalf("create first: %v", err)
	}
	_, err := repo.Create(ctx, newSession(owner, family, hash(2), nil))
	if err == nil {
		t.Fatal("a duplicate token hash was accepted")
	}
	if got := kindOf(t, err); got != apperr.KindConflict {
		t.Errorf("kind = %v, want Conflict", got)
	}
}

// TestRevokeFamilyRevokesEveryDescendant is the reuse-detection response. It
// has to reach the whole chain: revoking only the presented session would leave
// the thief's newest token working.
func TestRevokeFamilyRevokesEveryDescendant(t *testing.T) {
	pg := pgtest.Start(t)
	repo := authpg.NewSessionRepository(pg.Pool)
	ctx := context.Background()

	owner := seedUser(t, pg, "family@example.com")
	family := auth.FamilyID(user.NewID().UUID())

	first, err := repo.Create(ctx, newSession(owner, family, hash(3), nil))
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	_, err = repo.Create(ctx, newSession(owner, family, hash(4), &first.ID))
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	// A session from an unrelated login must survive.
	otherFamily := auth.FamilyID(user.NewID().UUID())
	untouched, err := repo.Create(ctx, newSession(owner, otherFamily, hash(5), nil))
	if err != nil {
		t.Fatalf("create unrelated: %v", err)
	}

	revoked, err := repo.RevokeFamily(ctx, family)
	if err != nil {
		t.Fatalf("revoke family: %v", err)
	}
	if revoked != 2 {
		t.Errorf("revoked %d sessions, want 2", revoked)
	}

	for _, id := range [][]byte{hash(3), hash(4)} {
		s, err := repo.GetByTokenHash(ctx, id)
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		if s.RevokedAt == nil {
			t.Errorf("session %v is still live after the family was revoked", s.ID)
		}
	}
	still, err := repo.GetByTokenHash(ctx, hash(5))
	if err != nil {
		t.Fatalf("reload unrelated: %v", err)
	}
	if still.RevokedAt != nil {
		t.Errorf("session %v from another family was revoked", untouched.ID)
	}

	// Revoking again affects nothing, which is what makes logout idempotent.
	again, err := repo.RevokeFamily(ctx, family)
	if err != nil {
		t.Fatalf("revoke family again: %v", err)
	}
	if again != 0 {
		t.Errorf("a second revocation touched %d rows, want 0", again)
	}
}

func TestSessionsRequireAnExistingUser(t *testing.T) {
	pg := pgtest.Start(t)
	repo := authpg.NewSessionRepository(pg.Pool)
	ctx := context.Background()

	orphan := user.User{ID: user.NewID()}
	family := auth.FamilyID(user.NewID().UUID())

	_, err := repo.Create(ctx, newSession(orphan, family, hash(6), nil))
	if err == nil {
		t.Fatal("a session was created for a user that does not exist")
	}
	if got := kindOf(t, err); got != apperr.KindConflict {
		t.Errorf("kind = %v, want Conflict", got)
	}
}

// TestDeletingAUserRemovesItsSessions verifies the cascade. Sessions that
// outlive their user would be rows nothing can ever revoke.
func TestDeletingAUserRemovesItsSessions(t *testing.T) {
	pg := pgtest.Start(t)
	sessions := authpg.NewSessionRepository(pg.Pool)
	users := userpg.New(pg.Pool)
	ctx := context.Background()

	owner := seedUser(t, pg, "cascade@example.com")
	family := auth.FamilyID(user.NewID().UUID())
	if _, err := sessions.Create(ctx, newSession(owner, family, hash(7), nil)); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := users.Delete(ctx, owner.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if _, err := sessions.GetByTokenHash(ctx, hash(7)); kindOf(t, err) != apperr.KindNotFound {
		t.Error("the session survived its owner")
	}
}

// TestWritesAcrossTwoRepositoriesShareOneTransaction is the demonstration
// required by the transaction design: the two repositories are separate types
// with separate pools-of-record, and the only thing joining them is the context.
// If either reached for the pool directly instead of the ambient transaction,
// the rollback below would leave a row behind.
func TestWritesAcrossTwoRepositoriesShareOneTransaction(t *testing.T) {
	pg := pgtest.Start(t)
	users := userpg.New(pg.Pool)
	sessions := authpg.NewSessionRepository(pg.Pool)
	tx := postgres.NewTxManager(pg.Pool)
	ctx := context.Background()

	sentinel := errors.New("deliberate failure after both writes")
	family := auth.FamilyID(user.NewID().UUID())

	err := tx.WithTx(ctx, func(ctx context.Context) error {
		created, err := users.Create(ctx, user.User{
			ID:           user.NewID(),
			Email:        "atomic@example.com",
			Name:         "Atomic",
			Status:       user.StatusActive,
			Role:         auth.RoleUser,
			PasswordHash: "$argon2id$v=19$m=65536,t=3,p=4$c2FsdHNhbHRzYWx0$aGFzaGhhc2hoYXNoaGFzaA",
		})
		if err != nil {
			return err
		}
		if _, err := sessions.Create(ctx, newSession(created, family, hash(8), nil)); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("WithTx returned %v, want the sentinel", err)
	}

	if _, err := users.GetByEmail(ctx, "atomic@example.com"); kindOf(t, err) != apperr.KindNotFound {
		t.Error("the user was committed despite the rollback")
	}
	if _, err := sessions.GetByTokenHash(ctx, hash(8)); kindOf(t, err) != apperr.KindNotFound {
		t.Error("the session was committed despite the rollback")
	}
}
