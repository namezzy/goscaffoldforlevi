//go:build integration

package main

import (
	"context"
	"errors"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/pgtest"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
	"github.com/namezzy/goscaffoldforlevi/internal/user/userpg"
)

// Cheap parameters. This test is about the bootstrap logic, not about the cost
// of Argon2, and the default 64 MiB / t=3 makes it needlessly slow.
var testParams = auth.Argon2Params{
	Time:    1,
	Memory:  8 * 1024,
	Threads: 1,
	SaltLen: auth.DefaultArgon2Params.SaltLen,
	KeyLen:  auth.DefaultArgon2Params.KeyLen,
}

func input(email, password string) user.CreateInput {
	return user.CreateInput{
		Email:    email,
		Name:     "Administrator",
		Password: password,
		Role:     auth.RoleSystemAdmin,
	}
}

// The point of the command is that the account it creates is usable. A test
// that only checked a row exists would still pass if the password were stored
// in a form nothing can verify, which is exactly the failure the manual
// procedure this command replaces was prone to.
func TestBootstrapCreatesAnAccountThePasswordVerifiesAgainst(t *testing.T) {
	pg := pgtest.Start(t)
	ctx := context.Background()

	password, err := generatePassword()
	if err != nil {
		t.Fatalf("generate a password: %v", err)
	}

	created, err := bootstrap(ctx, pg.Pool, testParams, input("Admin@Example.com ", password))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	if created.Role != auth.RoleSystemAdmin {
		t.Errorf("role = %q, want %q", created.Role, auth.RoleSystemAdmin)
	}
	if created.Status != user.StatusActive {
		t.Errorf("status = %q, want %q", created.Status, user.StatusActive)
	}
	// Going through the service is what normalises this. An INSERT-based
	// bootstrap would store the address as typed, and the account would then be
	// unreachable by a login that normalises.
	if created.Email != "admin@example.com" {
		t.Errorf("email = %q, want it normalised to %q", created.Email, "admin@example.com")
	}

	stored, err := userpg.New(pg.Pool).GetByEmail(ctx, created.Email)
	if err != nil {
		t.Fatalf("read the account back: %v", err)
	}

	hasher, err := auth.NewArgon2Hasher(testParams)
	if err != nil {
		t.Fatalf("build a hasher: %v", err)
	}
	if !hasher.Verify(password, stored.PasswordHash).OK {
		t.Fatal("the generated password does not verify against the stored hash")
	}
	if hasher.Verify(password+"x", stored.PasswordHash).OK {
		t.Fatal("a wrong password verified, so the check above proves nothing")
	}
}

// Without this refusal the command would be a permanent way to mint privileged
// accounts outside the API, bypassing authorization and the audit trail.
func TestBootstrapRefusesOnceAnAdministratorExists(t *testing.T) {
	pg := pgtest.Start(t)
	ctx := context.Background()

	if _, err := bootstrap(ctx, pg.Pool, testParams, input("first@example.com", "correct-horse-battery")); err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}

	_, err := bootstrap(ctx, pg.Pool, testParams, input("second@example.com", "correct-horse-battery"))
	if !errors.Is(err, ErrAdminExists) {
		t.Fatalf("second bootstrap error = %v, want ErrAdminExists", err)
	}

	if _, err := userpg.New(pg.Pool).GetByEmail(ctx, "second@example.com"); err == nil {
		t.Fatal("the refused bootstrap created an account anyway")
	}
}

// A user who is not an administrator must not unlock the command, or the
// refusal could be sidestepped by creating an ordinary account first.
func TestBootstrapIsNotBlockedByNonAdministrators(t *testing.T) {
	pg := pgtest.Start(t)
	ctx := context.Background()

	hasher, err := auth.NewArgon2Hasher(testParams)
	if err != nil {
		t.Fatalf("build a hasher: %v", err)
	}
	hash, err := hasher.Hash("correct-horse-battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	repo := userpg.New(pg.Pool)
	if _, err := repo.Create(ctx, user.User{
		ID:           user.NewID(),
		Email:        "member@example.com",
		Name:         "Member",
		Status:       user.StatusActive,
		Role:         auth.RoleReadOnly,
		PasswordHash: hash,
	}); err != nil {
		t.Fatalf("seed an ordinary user: %v", err)
	}

	if _, err := bootstrap(ctx, pg.Pool, testParams, input("admin@example.com", "correct-horse-battery")); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
}
