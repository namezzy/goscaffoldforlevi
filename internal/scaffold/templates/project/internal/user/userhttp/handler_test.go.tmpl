package userhttp_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/auth/authhttp"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/openapitest"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
	"github.com/namezzy/goscaffoldforlevi/internal/user/userhttp"
)

// These tests exercise the handlers through a real chi router so that URL
// parameters, method routing and the middleware wiring are covered too. A test
// that calls the handler function directly proves the body is parsed and
// nothing else; most of what can break at this layer is in the routing.

type fakeRepo struct {
	byID    map[user.ID]user.User
	byEmail map[string]user.ID
	order   []user.ID
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byID: map[user.ID]user.User{}, byEmail: map[string]user.ID{}}
}

func (f *fakeRepo) Create(_ context.Context, u user.User) (user.User, error) {
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

func (f *fakeRepo) GetByID(_ context.Context, id user.ID) (user.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return user.User{}, apperr.NotFound("USER_NOT_FOUND", "user not found")
	}
	return u, nil
}

func (f *fakeRepo) GetByEmail(_ context.Context, email string) (user.User, error) {
	id, ok := f.byEmail[email]
	if !ok {
		return user.User{}, apperr.NotFound("USER_NOT_FOUND", "user not found")
	}
	return f.byID[id], nil
}

func (f *fakeRepo) List(_ context.Context, limit, offset int32) ([]user.User, error) {
	var out []user.User
	for i, id := range f.order {
		if int32(i) < offset {
			continue
		}
		if int32(len(out)) == limit {
			break
		}
		out = append(out, f.byID[id])
	}
	return out, nil
}

func (f *fakeRepo) Count(context.Context) (int64, error) { return int64(len(f.order)), nil }

func (f *fakeRepo) Update(_ context.Context, id user.ID, p user.Patch) (user.User, error) {
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

func (f *fakeRepo) Delete(_ context.Context, id user.ID) error {
	if _, ok := f.byID[id]; !ok {
		return apperr.NotFound("USER_NOT_FOUND", "user not found")
	}
	delete(f.byID, id)
	return nil
}

type stubHasher struct{}

func (stubHasher) Hash(string) (string, error) { return "$stub$hash", nil }

// newServer builds the route tree with the same permission middleware the
// application uses, and injects the caller's principal from a header so that a
// test can act as any role without minting a token.
// directTx runs the function without a transaction. The handler tests are
// about status codes and response shapes, and the transactional guarantee is
// asserted where it lives, in the service and the repository tests.
type directTx struct{}

func (directTx) WithTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

func newServer(t *testing.T, repo *fakeRepo) http.Handler {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := userhttp.NewHandler(user.NewService(repo, stubHasher{}, directTx{}, messaging.NewMemoryPublisher(), log), log)

	r := chi.NewRouter()
	r.Use(principalFromHeader)
	r.Route("/users", h.Routes(func(p auth.Permission) func(http.Handler) http.Handler {
		return authhttp.RequirePermission(p, log)
	}))
	return r
}

// principalFromHeader reads "X-Test-Principal: <uuid>/<role>". An empty header
// leaves the request unauthenticated, which is how the 401 cases are produced.
func principalFromHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("X-Test-Principal")
		if raw == "" {
			next.ServeHTTP(w, r)
			return
		}
		id, role, _ := strings.Cut(raw, "/")
		uid, err := user.ParseID(id)
		if err != nil {
			http.Error(w, "bad test principal", http.StatusInternalServerError)
			return
		}
		ctx := auth.ContextWithPrincipal(r.Context(), auth.Principal{
			UserID: uid.String(),
			Role:   auth.Role(role),
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func seed(t *testing.T, repo *fakeRepo, email string, role auth.Role) user.User {
	t.Helper()

	u, err := repo.Create(context.Background(), user.User{
		ID:           user.NewID(),
		Email:        email,
		Name:         "Seeded",
		Status:       user.StatusActive,
		Role:         role,
		PasswordHash: "$stub$hash",
	})
	if err != nil {
		t.Fatalf("seed %s: %v", email, err)
	}
	return u
}

func do(t *testing.T, srv http.Handler, method, path, principal, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if principal != "" {
		req.Header.Set("X-Test-Principal", principal)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func asAdmin(u user.User) string { return u.ID.String() + "/" + string(auth.RoleAdmin) }
func asUser(u user.User) string  { return u.ID.String() + "/" + string(auth.RoleUser) }

func TestCreateUserAnswers201WithLocation(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	srv := newServer(t, repo)
	admin := seed(t, repo, "admin@example.com", auth.RoleAdmin)

	rec := do(t, srv, http.MethodPost, "/users", asAdmin(admin),
		`{"email":"New@Example.com","name":"New","password":"correct horse battery"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body)
	}

	var got map[string]any
	decode(t, rec, &got)

	// The Location header is what lets a client follow the creation without
	// parsing the body, and it must address the resource that was created.
	if want := "/api/v1/users/" + got["id"].(string); rec.Header().Get("Location") != want {
		t.Errorf("Location = %q, want %q", rec.Header().Get("Location"), want)
	}
	if got["email"] != "new@example.com" {
		t.Errorf("email = %v, want the normalised form", got["email"])
	}
	// Omitting role must produce the least privileged account, not an admin.
	if got["role"] != string(auth.RoleUser) {
		t.Errorf("role = %v, want user", got["role"])
	}
}

// TestResponsesNeverCarryThePasswordHash is the reason userResponse exists. The
// check is on the raw bytes rather than a decoded field, because the failure it
// guards against is a field appearing under some other name.
func TestResponsesNeverCarryThePasswordHash(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	srv := newServer(t, repo)
	admin := seed(t, repo, "admin@example.com", auth.RoleAdmin)

	for _, target := range []string{"/users", "/users/" + admin.ID.String()} {
		rec := do(t, srv, http.MethodGet, target, asAdmin(admin), "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d", target, rec.Code)
		}
		body := rec.Body.String()
		for _, leak := range []string{"password", "$stub$hash"} {
			if strings.Contains(strings.ToLower(body), leak) {
				t.Errorf("GET %s response contains %q: %s", target, leak, body)
			}
		}
	}
}

func TestListReturnsAnEmptyArrayNotNull(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	srv := newServer(t, repo)
	admin := seed(t, repo, "admin@example.com", auth.RoleAdmin)
	// Page two of a single-row table is empty.
	rec := do(t, srv, http.MethodGet, "/users?page=2&page_size=50", asAdmin(admin), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	// A client iterating the result should not have to null-check, so the
	// serialised form matters, not the decoded one.
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Errorf("empty page should serialise items as []: %s", rec.Body)
	}
}

func TestListRejectsAMalformedPageParameter(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	srv := newServer(t, repo)
	admin := seed(t, repo, "admin@example.com", auth.RoleAdmin)

	rec := do(t, srv, http.MethodGet, "/users?page=abc", asAdmin(admin), "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body)
	}
	if code := errorCode(t, rec); code != "INVALID_QUERY_PARAM" {
		t.Errorf("code = %q, want INVALID_QUERY_PARAM", code)
	}
}

func TestStatusCodesForTheCommonFailures(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	srv := newServer(t, repo)
	admin := seed(t, repo, "admin@example.com", auth.RoleAdmin)
	ordinary := seed(t, repo, "user@example.com", auth.RoleUser)
	missing := user.NewID().String()

	cases := []struct {
		name      string
		method    string
		path      string
		principal string
		body      string
		want      int
	}{
		{"no principal is 401", http.MethodGet, "/users/" + admin.ID.String(), "", "", http.StatusUnauthorized},
		{"listing without permission is 403", http.MethodGet, "/users", asUser(ordinary), "", http.StatusForbidden},
		{"reading another user is 403", http.MethodGet, "/users/" + admin.ID.String(), asUser(ordinary), "", http.StatusForbidden},
		{"deleting without permission is 403", http.MethodDelete, "/users/" + ordinary.ID.String(), asUser(ordinary), "", http.StatusForbidden},
		{"unknown id is 404", http.MethodGet, "/users/" + missing, asAdmin(admin), "", http.StatusNotFound},
		{"malformed id is 400", http.MethodGet, "/users/not-a-uuid", asAdmin(admin), "", http.StatusBadRequest},
		{
			"duplicate email is 409", http.MethodPost, "/users", asAdmin(admin),
			`{"email":"user@example.com","name":"Dup","password":"correct horse battery"}`,
			http.StatusConflict,
		},
		{
			"invalid body is 400", http.MethodPost, "/users", asAdmin(admin),
			`{"email":"not-an-email","name":"","password":"short"}`,
			http.StatusBadRequest,
		},
		{
			"empty patch is 400", http.MethodPatch, "/users/" + ordinary.ID.String(), asAdmin(admin),
			`{}`, http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, srv, tc.method, tc.path, tc.principal, tc.body)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body)
			}
		})
	}
}

// TestOwnerMayRenameItselfButNotPromoteItself is the privilege-escalation case:
// a self-service PATCH is legitimate, and it must not become a way to grant
// yourself a role.
func TestOwnerMayRenameItselfButNotPromoteItself(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	srv := newServer(t, repo)
	ordinary := seed(t, repo, "user@example.com", auth.RoleUser)
	path := "/users/" + ordinary.ID.String()

	rec := do(t, srv, http.MethodPatch, path, asUser(ordinary), `{"name":"Renamed"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("renaming self: status = %d, want 200: %s", rec.Code, rec.Body)
	}

	rec = do(t, srv, http.MethodPatch, path, asUser(ordinary), `{"role":"admin"}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("self-promotion: status = %d, want 403: %s", rec.Code, rec.Body)
	}
	if repo.byID[ordinary.ID].Role != auth.RoleUser {
		t.Error("the stored role changed despite the request being refused")
	}
}

func TestSuccessfulDeleteAndPasswordChangeReturn204WithNoBody(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	srv := newServer(t, repo)
	admin := seed(t, repo, "admin@example.com", auth.RoleAdmin)
	ordinary := seed(t, repo, "user@example.com", auth.RoleUser)

	rec := do(t, srv, http.MethodPut, "/users/"+ordinary.ID.String()+"/password",
		asUser(ordinary), `{"password":"another good passphrase"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("change password: status = %d, want 204: %s", rec.Code, rec.Body)
	}
	// A 204 with a body is a protocol violation, and some clients hang on it.
	if rec.Body.Len() != 0 {
		t.Errorf("204 carried a body: %q", rec.Body)
	}

	rec = do(t, srv, http.MethodDelete, "/users/"+ordinary.ID.String(), asAdmin(admin), "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status = %d, want 204: %s", rec.Code, rec.Body)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 carried a body: %q", rec.Body)
	}
	if _, still := repo.byID[ordinary.ID]; still {
		t.Error("the user survived a successful delete")
	}
}

// TestErrorsNeverNameThePermission checks that a refusal does not describe the
// permission model. Telling a caller which permission is missing maps out the
// authorization scheme for anyone probing it.
func TestErrorsNeverNameThePermission(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	srv := newServer(t, repo)
	ordinary := seed(t, repo, "user@example.com", auth.RoleUser)

	rec := do(t, srv, http.MethodGet, "/users", asUser(ordinary), "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if strings.Contains(rec.Body.String(), string(auth.PermUserList)) {
		t.Errorf("the 403 body names the missing permission: %s", rec.Body)
	}
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, into any) {
	t.Helper()

	if err := json.Unmarshal(rec.Body.Bytes(), into); err != nil {
		t.Fatalf("decode response: %v (%s)", err, rec.Body)
	}
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decode(t, rec, &body)
	return body.Error.Code
}

// TestResponseBodiesMatchTheDocumentedSchemas compares real handler output
// against api/openapi.yaml.
//
// The route test in cmd/api proves the paths agree. Nothing proved the bodies
// did: a field renamed in a DTO would keep every Go assertion passing, because
// they assert on the struct, while the document went on promising the old name
// to every generated client. This checks the bytes that actually leave the
// handler.
func TestResponseBodiesMatchTheDocumentedSchemas(t *testing.T) {
	t.Parallel()

	spec := openapitest.Load(t)
	repo := newFakeRepo()
	srv := newServer(t, repo)
	admin := seed(t, repo, "admin@example.com", auth.RoleAdmin)

	created := do(t, srv, http.MethodPost, "/users", asAdmin(admin),
		`{"email":"schema@example.com","name":"Schema","password":"correct horse battery"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: status = %d: %s", created.Code, created.Body)
	}
	spec.AssertMatches(t, "User", created.Body.Bytes())

	page := do(t, srv, http.MethodGet, "/users", asAdmin(admin), "")
	if page.Code != http.StatusOK {
		t.Fatalf("list: status = %d: %s", page.Code, page.Body)
	}
	// UserPage refers to User, so this also covers the nested representation,
	// which is the one a list client actually reads.
	spec.AssertMatches(t, "UserPage", page.Body.Bytes())

	notFound := do(t, srv, http.MethodGet, "/users/"+user.NewID().String(), asAdmin(admin), "")
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("get missing: status = %d: %s", notFound.Code, notFound.Body)
	}
	spec.AssertMatches(t, "Error", notFound.Body.Bytes())

	invalid := do(t, srv, http.MethodPost, "/users", asAdmin(admin),
		`{"email":"not-an-email","name":"","password":"short"}`)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid create: status = %d: %s", invalid.Code, invalid.Body)
	}
	// The validation failure is the only response that carries `details`, so it
	// is the only one that exercises that branch of the schema.
	spec.AssertMatches(t, "Error", invalid.Body.Bytes())
	if !strings.Contains(invalid.Body.String(), `"details"`) {
		t.Fatalf("expected a body with details, so that the schema check covers them: %s", invalid.Body)
	}
}
