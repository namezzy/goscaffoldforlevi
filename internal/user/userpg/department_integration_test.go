//go:build integration

package userpg_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/pgtest"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
	"github.com/namezzy/goscaffoldforlevi/internal/user/userpg"
)

// TestCreateStoresTheDepartment is the regression test for the gap a code
// review found: user.User.DepartmentID was never mapped into
// db.CreateUserParams, so invite.Service.Accept — which calls
// user.Service.ProvisionFromInvite with the invitation's DepartmentID set —
// silently created every invited member with no department at all,
// regardless of which department the invitation specified.
func TestCreateStoresTheDepartment(t *testing.T) {
	pg := pgtest.Start(t)
	ctx := context.Background()

	deptID := uuid.Must(uuid.NewV7())
	if _, err := pg.Pool.Exec(ctx,
		`INSERT INTO departments (id, name) VALUES ($1, $2)`, deptID, "Engineering",
	); err != nil {
		t.Fatalf("seed department: %v", err)
	}
	deptIDStr := deptID.String()

	repo := userpg.New(pg.Pool)
	created, err := repo.Create(ctx, user.User{
		ID: user.NewID(), Email: "with-department@example.com", Name: "With Department",
		Status: user.StatusActive, Role: auth.RoleReadOnly, PasswordHash: "$stub$hash",
		DepartmentID: &deptIDStr,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.DepartmentID == nil || *created.DepartmentID != deptIDStr {
		t.Fatalf("Create() DepartmentID = %v, want %q", created.DepartmentID, deptIDStr)
	}

	// Reread independently of Create's own return value, in case the bug
	// were in the RETURNING clause specifically rather than the INSERT.
	fetched, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fetched.DepartmentID == nil || *fetched.DepartmentID != deptIDStr {
		t.Fatalf("GetByID() DepartmentID = %v, want %q", fetched.DepartmentID, deptIDStr)
	}
}
