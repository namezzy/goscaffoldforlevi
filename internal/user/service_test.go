package user_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/example/go-enterprise/internal/apperr"
	"github.com/example/go-enterprise/internal/auth"
	"github.com/example/go-enterprise/internal/user"
)

// fakeRepository is an in-memory stand-in for the database.
//
// Writing it by hand rather than generating a mock keeps the test readable: the
// behaviour under test is the service's authorization and validation, and those
// are clearer against a store that actually stores things than against a list
// of expected calls.
type fakeRepository struct {
	byID     map[user.ID]user.User
	byEmail  map[string]user.ID
	order    []user.ID
	failWith error
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		byID:    make(map[user.ID]user.User),
		byEmail: make(map[string]user.ID),
	}
}

func (f *fakeRepository) Create(_ context.Context, u user.User) (user.User, error) {
	if f.failWith != nil {
		return user.User{}, f.failWith
	}
	// The real uniqueness guarantee is the database's unique index; the fake
	// mirrors it so that the service is exercised against the same contract.
	if _, taken := f.byEmail[u.Email]; taken {
		return user.User{}, apperr.Conflict("EMAIL_ALREADY_EXISTS", "a user with this email already exists")
	}
	u.CreatedAt = time.Now()
	u.UpdatedAt = u.CreatedAt
	f.byID[u.ID] = u
	f.byEmail[u.Email] = u.ID
	f.order = append([]user.ID{u.ID}, f.order...)
	return u, nil
}

func (f *fakeRepository) GetByID(_ context.Context, id user.ID) (user.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return user.User{}, apperr.NotFound("USER_NOT_FOUND", "user not found")
	}
	return u, nil
}

func (f *fakeRepository) GetByEmail(_ context.Context, email string) (user.User, error) {
	id, ok := f.byEmail[email]
	if !ok {
		return user.User{}, apperr.NotFound("USER_NOT_FOUND", "user not found")
	}
	return f.byID[id], nil
}

func (f *fakeRepository) List(_ context.Context, limit, offset int32) ([]user.User, error) {
	// PostgreSQL answers "OFFSET must not be negative" (SQLSTATE 2201X). A fake
	// that quietly accepted one could never fail when the service computed one,
	// which is precisely how an overflow bug survived until it was found by
	// hand against the real database.
	if offset < 0 || limit < 0 {
		return nil, errors.New("list users: OFFSET must not be negative")
	}
	out := []user.User{}
	for i := int(offset); i < len(f.order) && len(out) < int(limit); i++ {
		out = append(out, f.byID[f.order[i]])
	}
	return out, nil
}

func (f *fakeRepository) Count(context.Context) (int64, error) {
	return int64(len(f.order)), nil
}

func (f *fakeRepository) Update(_ context.Context, id user.ID, p user.Patch) (user.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return user.User{}, apperr.NotFound("USER_NOT_FOUND", "user not found")
	}
	if p.Name != nil {
		u.Name = *p.Name
	}
	if p.Status != nil {
		u.Status = *p.Status
	}
	if p.Role != nil {
		u.Role = *p.Role
	}
	if p.PasswordHash != nil {
		u.PasswordHash = *p.PasswordHash
	}
	u.UpdatedAt = time.Now()
	f.byID[id] = u
	return u, nil
}

func (f *fakeRepository) Delete(_ context.Context, id user.ID) error {
	u, ok := f.byID[id]
	if !ok {
		return apperr.NotFound("USER_NOT_FOUND", "user not found")
	}
	delete(f.byID, id)
	delete(f.byEmail, u.Email)
	for i, existing := range f.order {
		if existing == id {
			f.order = append(f.order[:i], f.order[i+1:]...)
			break
		}
	}
	return nil
}

// stubHasher avoids paying for a real Argon2 hash in every test. The hashing
// itself is covered in the auth package.
type stubHasher struct{}

func (stubHasher) Hash(password string) (string, error) { return "hashed:" + password, nil }

func newService(repo user.Repository) *user.Service {
	return user.NewService(repo, stubHasher{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func adminContext() context.Context {
	return auth.ContextWithPrincipal(context.Background(),
		auth.Principal{UserID: "admin-1", Role: auth.RoleAdmin})
}

func userContext(id user.ID) context.Context {
	return auth.ContextWithPrincipal(context.Background(),
		auth.Principal{UserID: id.String(), Role: auth.RoleUser})
}

const validPassword = "correct horse battery staple"

func seedUser(t *testing.T, repo *fakeRepository, email string, role auth.Role) user.User {
	t.Helper()
	u, err := newService(repo).Create(adminContext(), user.CreateInput{
		Email:    email,
		Name:     "Test User",
		Password: validPassword,
		Role:     role,
	})
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}
	return u
}

func TestCreateNormalisesEmailAndHashesThePassword(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	created := seedUser(t, repo, "  Alice@Example.COM ", auth.RoleUser)

	// Normalising at creation is what makes the unique index meaningful:
	// without it "a@x.com" and "A@x.com" are two accounts.
	if created.Email != "alice@example.com" {
		t.Errorf("Email = %q, want it normalised", created.Email)
	}
	if created.PasswordHash == validPassword {
		t.Fatal("the password was stored in plaintext")
	}
	if created.Status != user.StatusActive {
		t.Errorf("Status = %q, want active", created.Status)
	}
	if created.ID.IsZero() {
		t.Error("the user was created without an id")
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input user.CreateInput
	}{
		{"no email", user.CreateInput{Name: "A", Password: validPassword, Role: auth.RoleUser}},
		{"malformed email", user.CreateInput{Email: "not-an-email", Name: "A", Password: validPassword, Role: auth.RoleUser}},
		{"no name", user.CreateInput{Email: "a@example.com", Password: validPassword, Role: auth.RoleUser}},
		{"short password", user.CreateInput{Email: "a@example.com", Name: "A", Password: "short", Role: auth.RoleUser}},
		{"unknown role", user.CreateInput{Email: "a@example.com", Name: "A", Password: validPassword, Role: "root"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := newService(newFakeRepository()).Create(adminContext(), tc.input)
			if !apperr.Is(err, apperr.KindInvalidArgument) {
				t.Errorf("Create() kind = %v, want invalid_argument", apperr.KindOf(err))
			}
		})
	}
}

func TestCreateReportsDuplicateEmailAsAConflict(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	seedUser(t, repo, "a@example.com", auth.RoleUser)

	_, err := newService(repo).Create(adminContext(), user.CreateInput{
		Email: "A@EXAMPLE.com", Name: "Other", Password: validPassword, Role: auth.RoleUser,
	})
	if !apperr.Is(err, apperr.KindConflict) {
		t.Errorf("Create() kind = %v, want conflict", apperr.KindOf(err))
	}
}

func TestGetIsAllowedForTheOwnerAndForAdmins(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	target := seedUser(t, repo, "a@example.com", auth.RoleUser)
	other := seedUser(t, repo, "b@example.com", auth.RoleUser)
	svc := newService(repo)

	if _, err := svc.Get(userContext(target.ID), target.ID); err != nil {
		t.Errorf("the owner was denied their own record: %v", err)
	}
	if _, err := svc.Get(adminContext(), target.ID); err != nil {
		t.Errorf("an admin was denied: %v", err)
	}

	_, err := svc.Get(userContext(other.ID), target.ID)
	if !apperr.Is(err, apperr.KindForbidden) {
		t.Errorf("Get() kind = %v, want forbidden for another user's record", apperr.KindOf(err))
	}
	if _, err := svc.Get(context.Background(), target.ID); !apperr.Is(err, apperr.KindUnauthorized) {
		t.Errorf("Get() kind = %v, want unauthorized when anonymous", apperr.KindOf(err))
	}
}

// TestUpdateRefusesSelfServicePrivilegeEscalation is the reason role and status
// are singled out in Patch.ChangesPrivileges. Ownership is enough to rename
// yourself; if it were also enough to change your own role, every account would
// be one PATCH away from being an admin.
func TestUpdateRefusesSelfServicePrivilegeEscalation(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	target := seedUser(t, repo, "a@example.com", auth.RoleUser)
	svc := newService(repo)

	name := "New Name"
	if _, err := svc.Update(userContext(target.ID), target.ID, user.Patch{Name: &name}); err != nil {
		t.Fatalf("the owner could not rename themselves: %v", err)
	}

	admin := auth.RoleAdmin
	_, err := svc.Update(userContext(target.ID), target.ID, user.Patch{Role: &admin})
	if !apperr.Is(err, apperr.KindForbidden) {
		t.Fatalf("Update() kind = %v, want forbidden for a self role change", apperr.KindOf(err))
	}

	active := user.StatusActive
	if _, err := svc.Update(userContext(target.ID), target.ID, user.Patch{Status: &active}); !apperr.Is(err, apperr.KindForbidden) {
		t.Errorf("Update() kind = %v, want forbidden for a self status change", apperr.KindOf(err))
	}

	if _, err := svc.Update(adminContext(), target.ID, user.Patch{Role: &admin}); err != nil {
		t.Errorf("an admin could not change a role: %v", err)
	}
}

func TestUpdateRejectsAnEmptyPatch(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	target := seedUser(t, repo, "a@example.com", auth.RoleUser)

	// A PATCH that changes nothing is a client bug. Answering 200 would let it
	// go unnoticed.
	_, err := newService(repo).Update(adminContext(), target.ID, user.Patch{})
	if !apperr.Is(err, apperr.KindInvalidArgument) {
		t.Errorf("Update() kind = %v, want invalid_argument", apperr.KindOf(err))
	}
}

func TestUpdateRejectsUnknownEnumValues(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	target := seedUser(t, repo, "a@example.com", auth.RoleUser)

	bad := user.Status("deleted")
	_, err := newService(repo).Update(adminContext(), target.ID, user.Patch{Status: &bad})
	if !apperr.Is(err, apperr.KindInvalidArgument) {
		t.Errorf("Update() kind = %v, want invalid_argument", apperr.KindOf(err))
	}
}

func TestDeleteRequiresThePermissionEvenForTheOwner(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	target := seedUser(t, repo, "a@example.com", auth.RoleUser)
	svc := newService(repo)

	// Self-deletion is a product decision that has not been made. Defaulting to
	// allowing it would be irreversible.
	if err := svc.Delete(userContext(target.ID), target.ID); !apperr.Is(err, apperr.KindForbidden) {
		t.Errorf("Delete() kind = %v, want forbidden", apperr.KindOf(err))
	}
	if err := svc.Delete(adminContext(), target.ID); err != nil {
		t.Errorf("an admin could not delete: %v", err)
	}
}

func TestListRequiresThePermission(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	target := seedUser(t, repo, "a@example.com", auth.RoleUser)

	_, err := newService(repo).List(userContext(target.ID), 1, 20)
	if !apperr.Is(err, apperr.KindForbidden) {
		t.Errorf("List() kind = %v, want forbidden", apperr.KindOf(err))
	}
}

func TestListClampsPaging(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	for i := range 5 {
		seedUser(t, repo, string(rune('a'+i))+"@example.com", auth.RoleUser)
	}
	svc := newService(repo)

	cases := []struct {
		name         string
		page, size   int
		wantPage     int
		wantPageSize int
	}{
		// Clamping rather than rejecting: asking for a thousand rows is a
		// reasonable request and an unreasonable thing to serve, and an error
		// would be pedantic where a bound is sufficient.
		{"oversized page size", 1, 10000, 1, user.MaxPageSize},
		{"zero page size", 1, 0, 1, user.DefaultPageSize},
		{"negative page", -5, 20, 1, 20},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := svc.List(adminContext(), tc.page, tc.size)
			if err != nil {
				t.Fatalf("List() = %v", err)
			}
			if got.Page != tc.wantPage || got.PageSize != tc.wantPageSize {
				t.Errorf("List() page/size = %d/%d, want %d/%d",
					got.Page, got.PageSize, tc.wantPage, tc.wantPageSize)
			}
		})
	}
}

func TestListReportsTotals(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	for i := range 5 {
		seedUser(t, repo, string(rune('a'+i))+"@example.com", auth.RoleUser)
	}

	got, err := newService(repo).List(adminContext(), 1, 2)
	if err != nil {
		t.Fatalf("List() = %v", err)
	}
	if got.Total != 5 {
		t.Errorf("Total = %d, want 5", got.Total)
	}
	// Rounding up matters: with 5 items and a page size of 2 the last item is
	// on page 3, and truncating would tell the client it does not exist.
	if got.TotalPages != 3 {
		t.Errorf("TotalPages = %d, want 3", got.TotalPages)
	}
	if len(got.Items) != 2 {
		t.Errorf("len(Items) = %d, want 2", len(got.Items))
	}
}

func TestChangePasswordEnforcesThePolicyAndOwnership(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	target := seedUser(t, repo, "a@example.com", auth.RoleUser)
	other := seedUser(t, repo, "b@example.com", auth.RoleUser)
	svc := newService(repo)

	if err := svc.ChangePassword(userContext(target.ID), target.ID, "short"); !apperr.Is(err, apperr.KindInvalidArgument) {
		t.Errorf("ChangePassword() kind = %v, want invalid_argument", apperr.KindOf(err))
	}
	if err := svc.ChangePassword(userContext(other.ID), target.ID, validPassword); !apperr.Is(err, apperr.KindForbidden) {
		t.Errorf("ChangePassword() kind = %v, want forbidden", apperr.KindOf(err))
	}

	newPassword := strings.Repeat("z", 20)
	if err := svc.ChangePassword(userContext(target.ID), target.ID, newPassword); err != nil {
		t.Fatalf("ChangePassword() = %v", err)
	}
	stored, err := repo.GetByID(context.Background(), target.ID)
	if err != nil {
		t.Fatalf("GetByID() = %v", err)
	}
	if stored.PasswordHash != "hashed:"+newPassword {
		t.Error("the password hash was not replaced")
	}
}

func TestRepositoryErrorsPropagateUnchanged(t *testing.T) {
	t.Parallel()

	// The service must not reclassify what the repository decided; the
	// repository is the layer that knows what a driver error means.
	sentinel := apperr.Conflict("SOMETHING_SPECIFIC", "a specific conflict")
	repo := newFakeRepository()
	repo.failWith = sentinel

	_, err := newService(repo).Create(adminContext(), user.CreateInput{
		Email: "a@example.com", Name: "A", Password: validPassword, Role: auth.RoleUser,
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("Create() = %v, want the repository error unchanged", err)
	}
}

// TestNamesWithControlCharactersAreRejectedBeforeTheDatabase is the regression
// test for a 500 found by sending a name containing U+0000. PostgreSQL cannot
// store a NUL in a text column, so the write failed with SQLSTATE 22021 and the
// client was told the server had broken. The value is invalid input, and
// validation is both the layer that should reject it and the only layer that
// can say which field was wrong.
func TestNamesWithControlCharactersAreRejectedBeforeTheDatabase(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := newService(repo)
	ctx := adminContext()

	_, err := svc.Create(ctx, user.CreateInput{
		Email: "nul@example.com", Name: "A\x00B", Password: validPassword, Role: auth.RoleUser,
	})
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Kind != apperr.KindInvalidArgument {
		t.Fatalf("Create error = %v, want an invalid-argument error", err)
	}
	if len(repo.order) != 0 {
		t.Error("the invalid name still reached the repository")
	}

	existing := seedUser(t, repo, "patch@example.com", auth.RoleUser)
	bad := "A\x00B"
	_, err = svc.Update(ctx, existing.ID, user.Patch{Name: &bad})
	if !errors.As(err, &appErr) || appErr.Kind != apperr.KindInvalidArgument {
		t.Fatalf("Update error = %v, want an invalid-argument error", err)
	}
}

// TestAHugePageNumberCannotOverflowTheOffset is the regression test for a 500
// found by sending page=9223372036854775807.
//
// A guard existed and even documented this attack, but it tested for the
// overflow with a multiplication that overflowed first: page*pageSize wrapped
// to -20, which is not greater than the limit, so the guard passed and a
// negative OFFSET reached PostgreSQL. A bound has to be checked with arithmetic
// that cannot itself overflow.
func TestAHugePageNumberCannotOverflowTheOffset(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := newService(repo)
	ctx := adminContext()
	seedUser(t, repo, "a@example.com", auth.RoleUser)
	seedUser(t, repo, "b@example.com", auth.RoleUser)

	for _, page := range []int{math.MaxInt, math.MaxInt / 2, math.MaxInt / 20, 1 << 40, 1 << 31} {
		got, err := svc.List(ctx, page, 20)
		if err != nil {
			t.Fatalf("List(page=%d): %v", page, err)
		}
		if len(got.Items) != 0 {
			t.Errorf("List(page=%d) returned %d items; a page past the end is empty",
				page, len(got.Items))
		}
		if got.Total != 2 {
			t.Errorf("List(page=%d) total = %d, want 2", page, got.Total)
		}
	}
}
