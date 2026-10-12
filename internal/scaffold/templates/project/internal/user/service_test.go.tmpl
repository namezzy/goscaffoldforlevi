package user_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"maps"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
)

// fakeRepository is an in-memory stand-in for the database.
//
// Writing it by hand rather than generating a mock keeps the test readable: the
// behaviour under test is the service's authorization and validation, and those
// are clearer against a store that actually stores things than against a list
// of expected calls.
type fakeRepository struct {
	byID       map[user.ID]user.User
	byEmail    map[string]user.ID
	byUsername map[string]user.ID
	order      []user.ID
	failWith   error
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		byID:       make(map[user.ID]user.User),
		byEmail:    make(map[string]user.ID),
		byUsername: make(map[string]user.ID),
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
	if u.Username != "" {
		if _, taken := f.byUsername[u.Username]; taken {
			return user.User{}, apperr.Conflict("USERNAME_ALREADY_EXISTS", "this username is already taken")
		}
	}
	u.CreatedAt = time.Now()
	u.UpdatedAt = u.CreatedAt
	f.byID[u.ID] = u
	f.byEmail[u.Email] = u.ID
	if u.Username != "" {
		f.byUsername[u.Username] = u.ID
	}
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

func (f *fakeRepository) GetByIdentifier(_ context.Context, identifier string) (user.User, error) {
	if id, ok := f.byEmail[identifier]; ok {
		return f.byID[id], nil
	}
	if id, ok := f.byUsername[identifier]; ok {
		return f.byID[id], nil
	}
	return user.User{}, apperr.NotFound("USER_NOT_FOUND", "user not found")
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
	if p.Email != nil && *p.Email != u.Email {
		if _, taken := f.byEmail[*p.Email]; taken {
			return user.User{}, apperr.Conflict("EMAIL_ALREADY_EXISTS", "a user with this email already exists")
		}
		delete(f.byEmail, u.Email)
		u.Email = *p.Email
		f.byEmail[u.Email] = id
	}
	if p.Username != nil && *p.Username != u.Username {
		if _, taken := f.byUsername[*p.Username]; taken {
			return user.User{}, apperr.Conflict("USERNAME_ALREADY_EXISTS", "this username is already taken")
		}
		if u.Username != "" {
			delete(f.byUsername, u.Username)
		}
		u.Username = *p.Username
		f.byUsername[u.Username] = id
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
	if p.ClearDepartment {
		u.DepartmentID = nil
	} else if p.DepartmentID != nil {
		u.DepartmentID = p.DepartmentID
	}
	u.UpdatedAt = time.Now()
	f.byID[id] = u
	return u, nil
}

// ListFiltered and CountFiltered apply the same search/role/status
// predicate the real ListUsersFiltered query does, and sort by whichever
// column the caller asked for — a fake that ignored the filter would let a
// service-layer bug in ListFiltered's validation go untested.
func (f *fakeRepository) filtered(filter user.ListFilter) []user.User {
	search := strings.ToLower(filter.Search)
	out := make([]user.User, 0, len(f.order))
	for _, id := range f.order {
		u := f.byID[id]
		if search != "" &&
			!strings.Contains(strings.ToLower(u.Email), search) &&
			!strings.Contains(strings.ToLower(u.Username), search) &&
			!strings.Contains(strings.ToLower(u.Name), search) {
			continue
		}
		if filter.Role != nil && u.Role != *filter.Role {
			continue
		}
		if filter.Status != nil && u.Status != *filter.Status {
			continue
		}
		out = append(out, u)
	}
	less := func(a, b user.User) bool {
		switch filter.SortBy {
		case user.SortByName:
			return a.Name < b.Name
		case user.SortByEmail:
			return a.Email < b.Email
		default:
			return a.CreatedAt.Before(b.CreatedAt)
		}
	}
	slices.SortFunc(out, func(a, b user.User) int {
		switch {
		case less(a, b):
			if filter.SortDesc {
				return 1
			}
			return -1
		case less(b, a):
			if filter.SortDesc {
				return -1
			}
			return 1
		default:
			return 0
		}
	})
	return out
}

func (f *fakeRepository) ListFiltered(_ context.Context, filter user.ListFilter, limit, offset int32) ([]user.User, error) {
	all := f.filtered(filter)
	out := []user.User{}
	for i := int(offset); i < len(all) && len(out) < int(limit); i++ {
		out = append(out, all[i])
	}
	return out, nil
}

func (f *fakeRepository) CountFiltered(_ context.Context, filter user.ListFilter) (int64, error) {
	return int64(len(f.filtered(filter))), nil
}

// CountActiveByRole backs the last-system-admin guard.
func (f *fakeRepository) CountActiveByRole(_ context.Context, role auth.Role) (int64, error) {
	var n int64
	for _, u := range f.byID {
		if u.Role == role && u.Status == user.StatusActive {
			n++
		}
	}
	return n, nil
}

// LockActiveByRole has the same count-only behaviour as CountActiveByRole
// here: this fake has no notion of a database row lock, so it cannot
// reproduce the concurrency guarantee the real implementation provides —
// see TestConcurrentDemotionsCannotBothSucceed, which exercises that
// guarantee against a real PostgreSQL instance instead.
func (f *fakeRepository) LockActiveByRole(ctx context.Context, role auth.Role) (int64, error) {
	return f.CountActiveByRole(ctx, role)
}

func (f *fakeRepository) Delete(_ context.Context, id user.ID) error {
	u, ok := f.byID[id]
	if !ok {
		return apperr.NotFound("USER_NOT_FOUND", "user not found")
	}
	delete(f.byID, id)
	delete(f.byEmail, u.Email)
	if u.Username != "" {
		delete(f.byUsername, u.Username)
	}
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

// rollbackTx is a transaction manager that actually rolls back.
//
// A fake that merely calls fn is worse than no fake: it makes every test pass
// regardless of whether the code under test relies on rollback, and that is
// exactly how a reuse-detection bug in the auth service survived a green suite
// once already. Snapshotting the repository is enough to reproduce the only
// property the service depends on — a failed transaction leaves nothing behind.
type rollbackTx struct{ repo *fakeRepository }

func (t rollbackTx) WithTx(ctx context.Context, fn func(context.Context) error) error {
	if t.repo == nil {
		return fn(ctx)
	}

	byID := maps.Clone(t.repo.byID)
	byEmail := maps.Clone(t.repo.byEmail)
	order := slices.Clone(t.repo.order)

	if err := fn(ctx); err != nil {
		t.repo.byID, t.repo.byEmail, t.repo.order = byID, byEmail, order
		return err
	}
	return nil
}

func newService(repo user.Repository) *user.Service {
	fake, _ := repo.(*fakeRepository)
	return newServiceWithPublisher(repo, messaging.NewMemoryPublisher(), rollbackTx{repo: fake})
}

func newServiceWithPublisher(
	repo user.Repository, events user.EventPublisher, tx user.TxManager,
) *user.Service {
	return user.NewService(repo, stubHasher{}, tx, events,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func adminContext() context.Context {
	return auth.ContextWithPrincipal(context.Background(),
		auth.Principal{UserID: "admin-1", Role: auth.RoleSystemAdmin})
}

// userContext is a principal acting as themselves with no permission over
// anyone else's record — RoleOperator, of the five roles, is the one that
// holds none of the user:* permissions, which is what these tests need in
// order to exercise ownership (AuthorizeSelfOrPermission) rather than a
// granted permission. RoleReadOnly would not do: it can read and list any
// member, which is exactly the access these tests assert is absent.
func userContext(id user.ID) context.Context {
	return auth.ContextWithPrincipal(context.Background(),
		auth.Principal{UserID: id.String(), Role: auth.RoleOperator})
}

// memberAdminContext is a principal holding member_admin — user:update,
// user:create and role:assign, but deliberately not system_admin — for the
// tests that check what that role is still refused despite holding those
// permissions: granting or acting on a system_admin account.
func memberAdminContext() context.Context {
	return auth.ContextWithPrincipal(context.Background(),
		auth.Principal{UserID: "member-admin-1", Role: auth.RoleMemberAdmin})
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
	created := seedUser(t, repo, "  Alice@Example.COM ", auth.RoleReadOnly)

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
		{"no email", user.CreateInput{Name: "A", Password: validPassword, Role: auth.RoleReadOnly}},
		{"malformed email", user.CreateInput{Email: "not-an-email", Name: "A", Password: validPassword, Role: auth.RoleReadOnly}},
		{"no name", user.CreateInput{Email: "a@example.com", Password: validPassword, Role: auth.RoleReadOnly}},
		{"short password", user.CreateInput{Email: "a@example.com", Name: "A", Password: "short", Role: auth.RoleReadOnly}},
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
	seedUser(t, repo, "a@example.com", auth.RoleReadOnly)

	_, err := newService(repo).Create(adminContext(), user.CreateInput{
		Email: "A@EXAMPLE.com", Name: "Other", Password: validPassword, Role: auth.RoleReadOnly,
	})
	if !apperr.Is(err, apperr.KindConflict) {
		t.Errorf("Create() kind = %v, want conflict", apperr.KindOf(err))
	}
}

func TestCreateAcceptsAndNormalisesAnOptionalUsername(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	created, err := newService(repo).Create(adminContext(), user.CreateInput{
		Email: "a@example.com", Username: "  Ada_Lovelace  ", Name: "Ada",
		Password: validPassword, Role: auth.RoleReadOnly,
	})
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}
	if created.Username != "ada_lovelace" {
		t.Errorf("Username = %q, want it normalised", created.Username)
	}
}

func TestCreateWithoutAUsernameLeavesItUnset(t *testing.T) {
	t.Parallel()

	created := seedUser(t, newFakeRepository(), "a@example.com", auth.RoleReadOnly)
	if created.Username != "" {
		t.Errorf("Username = %q, want empty when none was supplied", created.Username)
	}
}

func TestCreateRejectsAMalformedUsername(t *testing.T) {
	t.Parallel()

	_, err := newService(newFakeRepository()).Create(adminContext(), user.CreateInput{
		Email: "a@example.com", Username: "1-not-a-valid-username", Name: "A",
		Password: validPassword, Role: auth.RoleReadOnly,
	})
	if !apperr.Is(err, apperr.KindInvalidArgument) {
		t.Errorf("Create() kind = %v, want invalid_argument", apperr.KindOf(err))
	}
}

func TestCreateReportsDuplicateUsernameAsAConflict(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	if _, err := newService(repo).Create(adminContext(), user.CreateInput{
		Email: "a@example.com", Username: "ada", Name: "Ada",
		Password: validPassword, Role: auth.RoleReadOnly,
	}); err != nil {
		t.Fatalf("seed Create() = %v", err)
	}

	_, err := newService(repo).Create(adminContext(), user.CreateInput{
		// A different email, but the same username spelled with different case
		// — normalisation must make the collision visible.
		Email: "b@example.com", Username: "ADA", Name: "Other",
		Password: validPassword, Role: auth.RoleReadOnly,
	})
	if !apperr.Is(err, apperr.KindConflict) {
		t.Errorf("Create() kind = %v, want conflict", apperr.KindOf(err))
	}
}

func TestGetIsAllowedForTheOwnerAndForAdmins(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	target := seedUser(t, repo, "a@example.com", auth.RoleReadOnly)
	other := seedUser(t, repo, "b@example.com", auth.RoleReadOnly)
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
	target := seedUser(t, repo, "a@example.com", auth.RoleReadOnly)
	svc := newService(repo)

	name := "New Name"
	if _, err := svc.Update(userContext(target.ID), target.ID, user.Patch{Name: &name}); err != nil {
		t.Fatalf("the owner could not rename themselves: %v", err)
	}

	admin := auth.RoleSystemAdmin
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

// TestUpdateEmailAndUsernameAreOwnerEditable mirrors
// TestUpdateRefusesSelfServicePrivilegeEscalation for the two fields this
// change adds: unlike role and status, changing your own email or username
// grants no capability, so ownership alone is enough — the same rule that
// already applies to name.
func TestUpdateEmailAndUsernameAreOwnerEditable(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	target := seedUser(t, repo, "a@example.com", auth.RoleReadOnly)
	svc := newService(repo)

	newEmail := "  New@Example.COM  "
	username := "  Ada_Lovelace  "
	updated, err := svc.Update(userContext(target.ID), target.ID, user.Patch{
		Email: &newEmail, Username: &username,
	})
	if err != nil {
		t.Fatalf("the owner could not change their own email/username: %v", err)
	}
	if updated.Email != "new@example.com" {
		t.Errorf("Email = %q, want it normalised", updated.Email)
	}
	if updated.Username != "ada_lovelace" {
		t.Errorf("Username = %q, want it normalised", updated.Username)
	}
}

func TestUpdateRejectsAMalformedEmailOrUsername(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	target := seedUser(t, repo, "a@example.com", auth.RoleReadOnly)
	svc := newService(repo)

	badEmail := "not-an-email"
	if _, err := svc.Update(adminContext(), target.ID, user.Patch{Email: &badEmail}); !apperr.Is(err, apperr.KindInvalidArgument) {
		t.Errorf("Update() kind = %v, want invalid_argument for a malformed email", apperr.KindOf(err))
	}

	badUsername := "1-not-valid"
	if _, err := svc.Update(adminContext(), target.ID, user.Patch{Username: &badUsername}); !apperr.Is(err, apperr.KindInvalidArgument) {
		t.Errorf("Update() kind = %v, want invalid_argument for a malformed username", apperr.KindOf(err))
	}
}

// TestUpdateReportsDuplicateEmailOrUsernameAsAConflict is the case the user's
// request named explicitly: before a username change takes effect, it must
// be checked against every other account, and the check must be atomic with
// the write — see fakeRepository.Update, which mirrors the real unique
// index rather than pre-checking with a separate read.
func TestUpdateReportsDuplicateEmailOrUsernameAsAConflict(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	taken := seedUser(t, repo, "taken@example.com", auth.RoleReadOnly)
	if _, err := newService(repo).Update(adminContext(), taken.ID, user.Patch{Username: ptr("ada")}); err != nil {
		t.Fatalf("seed username: %v", err)
	}
	target := seedUser(t, repo, "a@example.com", auth.RoleReadOnly)
	svc := newService(repo)

	_, err := svc.Update(userContext(target.ID), target.ID, user.Patch{Email: &taken.Email})
	if !apperr.Is(err, apperr.KindConflict) {
		t.Errorf("Update() kind = %v, want conflict for a taken email", apperr.KindOf(err))
	}

	_, err = svc.Update(userContext(target.ID), target.ID, user.Patch{Username: ptr("ADA")})
	if !apperr.Is(err, apperr.KindConflict) {
		t.Errorf("Update() kind = %v, want conflict for a taken username (case-insensitive)", apperr.KindOf(err))
	}
}

func ptr(s string) *string { return &s }

func TestUpdateRejectsAnEmptyPatch(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	target := seedUser(t, repo, "a@example.com", auth.RoleReadOnly)

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
	target := seedUser(t, repo, "a@example.com", auth.RoleReadOnly)

	bad := user.Status("deleted")
	_, err := newService(repo).Update(adminContext(), target.ID, user.Patch{Status: &bad})
	if !apperr.Is(err, apperr.KindInvalidArgument) {
		t.Errorf("Update() kind = %v, want invalid_argument", apperr.KindOf(err))
	}
}

func TestDeleteRequiresThePermissionEvenForTheOwner(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	target := seedUser(t, repo, "a@example.com", auth.RoleReadOnly)
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
	target := seedUser(t, repo, "a@example.com", auth.RoleReadOnly)

	_, err := newService(repo).List(userContext(target.ID), 1, 20)
	if !apperr.Is(err, apperr.KindForbidden) {
		t.Errorf("List() kind = %v, want forbidden", apperr.KindOf(err))
	}
}

func TestListClampsPaging(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	for i := range 5 {
		seedUser(t, repo, string(rune('a'+i))+"@example.com", auth.RoleReadOnly)
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
		seedUser(t, repo, string(rune('a'+i))+"@example.com", auth.RoleReadOnly)
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
	target := seedUser(t, repo, "a@example.com", auth.RoleReadOnly)
	other := seedUser(t, repo, "b@example.com", auth.RoleReadOnly)
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
		Email: "a@example.com", Name: "A", Password: validPassword, Role: auth.RoleReadOnly,
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
		Email: "nul@example.com", Name: "A\x00B", Password: validPassword, Role: auth.RoleReadOnly,
	})
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Kind != apperr.KindInvalidArgument {
		t.Fatalf("Create error = %v, want an invalid-argument error", err)
	}
	if len(repo.order) != 0 {
		t.Error("the invalid name still reached the repository")
	}

	existing := seedUser(t, repo, "patch@example.com", auth.RoleReadOnly)
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
	seedUser(t, repo, "a@example.com", auth.RoleReadOnly)
	seedUser(t, repo, "b@example.com", auth.RoleReadOnly)

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

// --- Role-escalation guards (auth.GuardRoleGrant / auth.GuardActionOnAccount) ---

// TestMemberAdminCannotCreateASystemAdmin is the regression test for the
// escalation path found in review: member_admin holds user:create, and
// nothing beyond the ordinary permission check used to stop it choosing
// role: system_admin for the account it was creating.
func TestMemberAdminCannotCreateASystemAdmin(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := newService(repo)

	_, err := svc.Create(memberAdminContext(), user.CreateInput{
		Email: "new-admin@example.com", Name: "New Admin", Password: validPassword,
		Role: auth.RoleSystemAdmin,
	})
	if !apperr.Is(err, apperr.KindForbidden) {
		t.Fatalf("Create() kind = %v, want forbidden", apperr.KindOf(err))
	}
	if len(repo.byID) != 0 {
		t.Error("a system_admin account was created despite the refusal")
	}

	// The same call succeeds for an actual system administrator, so this is
	// specifically about who is asking, not a blanket refusal.
	created, err := svc.Create(adminContext(), user.CreateInput{
		Email: "new-admin@example.com", Name: "New Admin", Password: validPassword,
		Role: auth.RoleSystemAdmin,
	})
	if err != nil {
		t.Fatalf("Create() by a system_admin = %v, want nil", err)
	}
	if created.Role != auth.RoleSystemAdmin {
		t.Errorf("role = %q, want system_admin", created.Role)
	}
}

// TestMemberAdminCannotPromoteToSystemAdmin covers the same guard on the
// Update path, including a member_admin attempting to promote themselves.
func TestMemberAdminCannotPromoteToSystemAdmin(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := newService(repo)
	target := seedUser(t, repo, "target@example.com", auth.RoleReadOnly)

	admin := auth.RoleSystemAdmin
	if _, err := svc.Update(memberAdminContext(), target.ID, user.Patch{Role: &admin}); !apperr.Is(err, apperr.KindForbidden) {
		t.Errorf("Update() promoting another account, kind = %v, want forbidden", apperr.KindOf(err))
	}
	if repo.byID[target.ID].Role != auth.RoleReadOnly {
		t.Error("the stored role changed despite the refusal")
	}

	// A member_admin acting on their own account cannot self-promote either
	// — role:assign plus ownership must not add up to an escalation the
	// permission alone does not grant.
	memberAdminID := seedUser(t, repo, "member-admin@example.com", auth.RoleMemberAdmin)
	selfCtx := auth.ContextWithPrincipal(context.Background(),
		auth.Principal{UserID: memberAdminID.ID.String(), Role: auth.RoleMemberAdmin})
	if _, err := svc.Update(selfCtx, memberAdminID.ID, user.Patch{Role: &admin}); !apperr.Is(err, apperr.KindForbidden) {
		t.Errorf("Update() self-promotion, kind = %v, want forbidden", apperr.KindOf(err))
	}
}

// TestMemberAdminCannotActOnAnExistingSystemAdmin covers Update (a
// non-privilege field), ChangePassword and Delete: a member_admin holding
// the ordinary permission each of those checks must still be refused when
// the target already holds system_admin.
func TestMemberAdminCannotActOnAnExistingSystemAdmin(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) (*fakeRepository, user.User) {
		t.Helper()
		repo := newFakeRepository()
		target := seedUser(t, repo, "target-admin@example.com", auth.RoleSystemAdmin)
		return repo, target
	}

	t.Run("update", func(t *testing.T) {
		t.Parallel()
		repo, target := setup(t)
		newName := "Renamed"
		_, err := newService(repo).Update(memberAdminContext(), target.ID, user.Patch{Name: &newName})
		if !apperr.Is(err, apperr.KindForbidden) {
			t.Errorf("Update() kind = %v, want forbidden", apperr.KindOf(err))
		}
	})

	t.Run("change password", func(t *testing.T) {
		t.Parallel()
		repo, target := setup(t)
		err := newService(repo).ChangePassword(memberAdminContext(), target.ID, "a different password")
		if !apperr.Is(err, apperr.KindForbidden) {
			t.Errorf("ChangePassword() kind = %v, want forbidden", apperr.KindOf(err))
		}
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()
		repo, target := setup(t)
		// A second active system admin, so this cannot be refused by the
		// last-admin guard instead — this test is specifically about the
		// account-control guard.
		seedUser(t, repo, "other-admin@example.com", auth.RoleSystemAdmin)
		err := newService(repo).Delete(memberAdminContext(), target.ID)
		if !apperr.Is(err, apperr.KindForbidden) {
			t.Errorf("Delete() kind = %v, want forbidden", apperr.KindOf(err))
		}
		if _, ok := repo.byID[target.ID]; !ok {
			t.Error("the account was deleted despite the refusal")
		}
	})
}

// --- Last-system-admin protection ---

func TestUpdateProtectsTheLastActiveSystemAdmin(t *testing.T) {
	t.Parallel()

	t.Run("demoting the only active admin is refused", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepository()
		onlyAdmin := seedUser(t, repo, "only-admin@example.com", auth.RoleSystemAdmin)
		readOnly := auth.RoleReadOnly
		_, err := newService(repo).Update(adminContext(), onlyAdmin.ID, user.Patch{Role: &readOnly})
		if !apperr.Is(err, apperr.KindConflict) {
			t.Fatalf("Update() kind = %v, want conflict", apperr.KindOf(err))
		}
		if repo.byID[onlyAdmin.ID].Role != auth.RoleSystemAdmin {
			t.Error("the role changed despite the refusal")
		}
	})

	t.Run("disabling the only active admin is refused", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepository()
		onlyAdmin := seedUser(t, repo, "only-admin@example.com", auth.RoleSystemAdmin)
		disabled := user.StatusDisabled
		_, err := newService(repo).Update(adminContext(), onlyAdmin.ID, user.Patch{Status: &disabled})
		if !apperr.Is(err, apperr.KindConflict) {
			t.Fatalf("Update() kind = %v, want conflict", apperr.KindOf(err))
		}
	})

	t.Run("demoting one of two active admins is allowed", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepository()
		first := seedUser(t, repo, "first-admin@example.com", auth.RoleSystemAdmin)
		seedUser(t, repo, "second-admin@example.com", auth.RoleSystemAdmin)
		readOnly := auth.RoleReadOnly
		if _, err := newService(repo).Update(adminContext(), first.ID, user.Patch{Role: &readOnly}); err != nil {
			t.Errorf("Update() = %v, want nil", err)
		}
	})

	t.Run("demoting a non-admin never checks the guard", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepository()
		onlyAdmin := seedUser(t, repo, "only-admin@example.com", auth.RoleSystemAdmin)
		member := seedUser(t, repo, "member@example.com", auth.RoleReadOnly)
		operator := auth.RoleOperator
		if _, err := newService(repo).Update(adminContext(), member.ID, user.Patch{Role: &operator}); err != nil {
			t.Errorf("Update() = %v, want nil (unrelated to the sole system admin)", err)
		}
		if repo.byID[onlyAdmin.ID].Role != auth.RoleSystemAdmin {
			t.Error("the unrelated update somehow touched the admin's role")
		}
	})
}

func TestDeleteProtectsTheLastActiveSystemAdmin(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	onlyAdmin := seedUser(t, repo, "only-admin@example.com", auth.RoleSystemAdmin)
	err := newService(repo).Delete(adminContext(), onlyAdmin.ID)
	if !apperr.Is(err, apperr.KindConflict) {
		t.Fatalf("Delete() kind = %v, want conflict", apperr.KindOf(err))
	}
	if _, ok := repo.byID[onlyAdmin.ID]; !ok {
		t.Error("the account was deleted despite the refusal")
	}
}

// --- Department ownership and clearing (self-service vs. user:update) ---

// TestSelfServiceCannotChangeOwnDepartment is the regression test for the
// gap where DepartmentID was missing from Patch.ChangesPrivileges: a
// read_only member's own PATCH containing only department_id fell through
// to the self-service authorization tier, which ownership alone satisfies.
func TestSelfServiceCannotChangeOwnDepartment(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	target := seedUser(t, repo, "member@example.com", auth.RoleReadOnly)
	deptID := "11111111-1111-1111-1111-111111111111"

	_, err := newService(repo).Update(userContext(target.ID), target.ID, user.Patch{DepartmentID: &deptID})
	if !apperr.Is(err, apperr.KindForbidden) {
		t.Fatalf("Update() kind = %v, want forbidden", apperr.KindOf(err))
	}
	if repo.byID[target.ID].DepartmentID != nil {
		t.Error("the department changed despite the refusal")
	}

	// The same field is exactly what user:update is for.
	if _, err := newService(repo).Update(adminContext(), target.ID, user.Patch{DepartmentID: &deptID}); err != nil {
		t.Errorf("Update() by an admin = %v, want nil", err)
	}
}

// TestUpdateCanClearADepartment is the regression test for
// Patch.ClearDepartment: a bare nil DepartmentID cannot mean "clear", since
// that is indistinguishable from "not provided" — this is the field that
// makes clearing possible at all.
func TestUpdateCanClearADepartment(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	target := seedUser(t, repo, "member@example.com", auth.RoleReadOnly)
	deptID := "11111111-1111-1111-1111-111111111111"
	if _, err := newService(repo).Update(adminContext(), target.ID, user.Patch{DepartmentID: &deptID}); err != nil {
		t.Fatalf("assign department: %v", err)
	}
	if repo.byID[target.ID].DepartmentID == nil {
		t.Fatal("assign department: not stored")
	}

	updated, err := newService(repo).Update(adminContext(), target.ID, user.Patch{ClearDepartment: true})
	if err != nil {
		t.Fatalf("clear department: %v", err)
	}
	if updated.DepartmentID != nil {
		t.Errorf("DepartmentID = %v, want nil after clearing", updated.DepartmentID)
	}
	if repo.byID[target.ID].DepartmentID != nil {
		t.Error("the stored row still carries a department after clearing")
	}
}
