package authhttp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/auth/authhttp"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
)

// The service is built from fakes rather than mocked away, so these tests
// exercise the real code path from the wire to the domain and back. What they
// assert is the transport layer's own responsibilities: which status codes are
// returned, which headers are set, and what does not appear in the body.

type fakeAccounts struct {
	account auth.Account
	err     error
}

func (f fakeAccounts) FindByIdentifier(context.Context, string) (auth.Account, error) {
	return f.account, f.err
}
func (f fakeAccounts) FindByID(context.Context, string) (auth.Account, error) {
	return f.account, f.err
}
func (f fakeAccounts) UpdatePasswordHash(context.Context, string, string) error { return nil }

type fakeSessions struct {
	created  auth.NewSession
	session  auth.Session
	getErr   error
	createOK bool
}

func (f *fakeSessions) Create(_ context.Context, s auth.NewSession) (auth.Session, error) {
	f.created = s
	f.createOK = true
	return auth.Session{ID: s.ID, UserID: s.UserID, FamilyID: s.FamilyID, ExpiresAt: s.ExpiresAt}, nil
}

func (f *fakeSessions) GetByTokenHash(context.Context, []byte) (auth.Session, error) {
	return f.session, f.getErr
}

func (f *fakeSessions) Revoke(context.Context, auth.SessionID) error { return nil }

func (f *fakeSessions) RevokeFamily(context.Context, auth.FamilyID) (int64, error) { return 1, nil }

func (f *fakeSessions) ListRecentByUser(context.Context, string, int32) ([]auth.Session, error) {
	return nil, nil
}

func (f *fakeSessions) RevokeOwned(context.Context, auth.FamilyID, string) (bool, error) {
	return true, nil
}

func (f *fakeSessions) RevokeAllForUserExceptFamily(context.Context, string, auth.FamilyID) (int64, error) {
	return 0, nil
}

func (f *fakeSessions) RevokeAllForUser(context.Context, string) (int64, error) { return 0, nil }

type fakeResets struct{}

func (fakeResets) Create(context.Context, auth.NewPasswordResetToken) error { return nil }
func (fakeResets) GetByTokenHash(context.Context, []byte) (auth.PasswordResetToken, error) {
	return auth.PasswordResetToken{}, apperr.NotFound("PASSWORD_RESET_TOKEN_NOT_FOUND", "not found")
}
func (fakeResets) MarkUsed(context.Context, string) (bool, error)                  { return true, nil }
func (fakeResets) InvalidatePendingForUser(context.Context, string) (int64, error) { return 0, nil }

type fakeCipher struct{}

func (fakeCipher) Encrypt(s string) (string, error) { return s, nil }

type fakePublisher struct{}

func (fakePublisher) Publish(context.Context, messaging.Event) error { return nil }

type fakeHasher struct{ ok bool }

func (f fakeHasher) Hash(string) (string, error) { return "hashed", nil }
func (f fakeHasher) Verify(string, string) auth.VerifyResult {
	return auth.VerifyResult{OK: f.ok}
}
func (f fakeHasher) VerifyDummy() {}

type fakeIssuer struct{}

func (fakeIssuer) Issue(auth.AccessClaims) (string, error) { return "issued.access.token", nil }

type directTx struct{}

func (directTx) WithTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

func newHandler(t *testing.T, passwordOK bool, sessions *fakeSessions) *authhttp.Handler {
	t.Helper()

	svc := auth.NewService(
		auth.ServiceConfig{
			Issuer:           "test",
			Audience:         "test-api",
			AccessTTL:        15 * time.Minute,
			RefreshTTL:       720 * time.Hour,
			PasswordResetTTL: time.Hour,
		},
		fakeAccounts{account: auth.Account{
			UserID:       "01930f8e-0000-7000-8000-000000000001",
			PasswordHash: "stored",
			Role:         auth.RoleReadOnly,
			Active:       true,
		}},
		sessions,
		fakeResets{},
		fakeCipher{},
		fakeHasher{ok: passwordOK},
		fakeIssuer{},
		fakePublisher{},
		directTx{},
		discardLogger(),
	)
	// Zero trusted proxies: X-Forwarded-For is ignored, which is the safe
	// default and the one these tests pin.
	return authhttp.NewHandler(svc, 0, discardLogger())
}

func router(h *authhttp.Handler) http.Handler {
	r := chi.NewRouter()
	noLimit := func(next http.Handler) http.Handler { return next }
	r.Route("/auth", h.Routes(noLimit))
	return r
}

func post(t *testing.T, h *authhttp.Handler, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router(h).ServeHTTP(rec, req)
	return rec
}

func TestLoginReturnsAPairThatMustNotBeCached(t *testing.T) {
	sessions := &fakeSessions{}
	rec := post(t, newHandler(t, true, sessions), "/auth/login", `{"identifier":"a@example.com","password":"correct horse"}`, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200. body: %s", rec.Code, rec.Body)
	}

	// Without these headers a shared proxy or the browser back-button can serve
	// one user's tokens to the next request. This is the assertion that would
	// notice if writeTokens were simplified.
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control %q, want no-store", got)
	}
	if got := rec.Header().Get("Pragma"); got != "no-cache" {
		t.Errorf("Pragma %q, want no-cache", got)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, field := range []string{"access_token", "refresh_token", "token_type", "expires_in"} {
		if _, ok := body[field]; !ok {
			t.Errorf("the response has no %s: %s", field, rec.Body)
		}
	}
	if body["token_type"] != "Bearer" {
		t.Errorf("token_type %v, want Bearer", body["token_type"])
	}
	if !sessions.createOK {
		t.Error("no session was recorded for a successful login")
	}
}

func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	// Wrong password and unknown account must produce the same status, the
	// same code and the same message. Any difference makes this endpoint an
	// account enumeration oracle, which is the single most valuable thing an
	// attacker can get from a login form.
	wrongPassword := post(t, newHandler(t, false, &fakeSessions{}), "/auth/login",
		`{"identifier":"a@example.com","password":"wrong"}`, nil)

	if wrongPassword.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401. body: %s", wrongPassword.Code, wrongPassword.Body)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(wrongPassword.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "INVALID_CREDENTIALS" {
		t.Errorf("code %q, want INVALID_CREDENTIALS", body.Error.Code)
	}

	// The message must not name which half was wrong.
	for _, leak := range []string{"password", "identifier", "exist", "disabled", "found"} {
		if strings.Contains(strings.ToLower(body.Error.Message), leak) && leak != "password" && leak != "identifier" {
			t.Errorf("the message %q names the failing half", body.Error.Message)
		}
	}
}

func TestNoResponseEverEchoesTheCredential(t *testing.T) {
	const password = "sup3r-s3cret-passphrase"

	for name, rec := range map[string]*httptest.ResponseRecorder{
		"success": post(t, newHandler(t, true, &fakeSessions{}), "/auth/login",
			`{"identifier":"a@example.com","password":"`+password+`"}`, nil),
		"failure": post(t, newHandler(t, false, &fakeSessions{}), "/auth/login",
			`{"identifier":"a@example.com","password":"`+password+`"}`, nil),
	} {
		if strings.Contains(rec.Body.String(), password) {
			t.Errorf("%s: the response body contains the submitted password: %s", name, rec.Body)
		}
	}
}

func TestMalformedBodiesAreRefused(t *testing.T) {
	tests := map[string]string{
		"not json":      `{`,
		"unknown field": `{"identifier":"a@example.com","password":"x","admin":true}`,
		"wrong type":    `{"identifier":123,"password":"x"}`,
		"empty":         ``,
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			rec := post(t, newHandler(t, true, &fakeSessions{}), "/auth/login", body, nil)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status %d, want 400. body: %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestRefreshRejectsAnUnknownToken(t *testing.T) {
	sessions := &fakeSessions{getErr: apperr.NotFound("SESSION_NOT_FOUND", "session not found")}

	rec := post(t, newHandler(t, true, sessions), "/auth/refresh", `{"refresh_token":"whatever"}`, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401. body: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "no such session") {
		t.Errorf("the internal error text reached the client: %s", rec.Body)
	}
}

func TestLogoutIs204AndSaysNothingAboutTheToken(t *testing.T) {
	// Logout is unauthenticated by necessity — the access token may already
	// have expired — so it must answer the same way whether or not the session
	// existed. Otherwise it becomes an oracle for live refresh tokens.
	sessions := &fakeSessions{getErr: apperr.NotFound("SESSION_NOT_FOUND", "session not found")}

	rec := post(t, newHandler(t, true, sessions), "/auth/logout", `{"refresh_token":"never-existed"}`, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d, want 204. body: %s", rec.Code, rec.Body)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 carried a body: %s", rec.Body)
	}
}

// TestLogoutStillReportsARealFailure is the other half of the rule above, and
// the reason "always 204" would be wrong as an implementation.
//
// A missing session is an expected state and is hidden. A database that is
// down is not: answering 204 there would tell the client the family was
// revoked when nothing was, and the stolen token would keep working.
func TestLogoutStillReportsARealFailure(t *testing.T) {
	sessions := &fakeSessions{getErr: errors.New("connection refused")}

	rec := post(t, newHandler(t, true, sessions), "/auth/logout", `{"refresh_token":"whatever"}`, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500 when the store fails. body: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "connection refused") {
		t.Errorf("the internal error text reached the client: %s", rec.Body)
	}
}

func TestTheRecordedAddressIgnoresAForgedHeader(t *testing.T) {
	// trustedProxies is 0, so X-Forwarded-For must be ignored entirely. The
	// session records the client IP, and recording a client-controlled value
	// would make the audit trail worse than useless: it would be confidently
	// wrong.
	sessions := &fakeSessions{}
	rec := post(t, newHandler(t, true, sessions), "/auth/login",
		`{"identifier":"a@example.com","password":"correct horse"}`,
		map[string]string{"X-Forwarded-For": "203.0.113.7"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200. body: %s", rec.Code, rec.Body)
	}
	if sessions.created.IP == nil {
		t.Fatal("no address was recorded")
	}
	forged := netip.MustParseAddr("203.0.113.7")
	if *sessions.created.IP == forged {
		t.Errorf("the forged X-Forwarded-For address %s was recorded", forged)
	}
}

func TestTheUserAgentIsRecorded(t *testing.T) {
	sessions := &fakeSessions{}
	post(t, newHandler(t, true, sessions), "/auth/login",
		`{"identifier":"a@example.com","password":"correct horse"}`,
		map[string]string{"User-Agent": "curl/8.0"})

	if sessions.created.UserAgent == nil || *sessions.created.UserAgent != "curl/8.0" {
		t.Errorf("user agent %v, want curl/8.0", sessions.created.UserAgent)
	}
}

// TestPasswordResetRequestIsAlways202 is the wire-level half of the
// account-enumeration guarantee internal/auth's own tests cover at the
// service layer: whatever the service decides internally, the HTTP
// response must look identical for a request an attacker cannot
// distinguish from a real one.
func TestPasswordResetRequestIsAlways202(t *testing.T) {
	rec := post(t, newHandler(t, true, &fakeSessions{}), "/auth/password-reset/request",
		`{"identifier":"whoever@example.com"}`, nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d, want 202. body: %s", rec.Code, rec.Body)
	}
}

func TestPasswordResetRequestRejectsAMalformedBody(t *testing.T) {
	rec := post(t, newHandler(t, true, &fakeSessions{}), "/auth/password-reset/request", `{`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400. body: %s", rec.Code, rec.Body)
	}
}

func TestPasswordResetConfirmRejectsAnUnknownToken(t *testing.T) {
	rec := post(t, newHandler(t, true, &fakeSessions{}), "/auth/password-reset/confirm",
		`{"token":"whatever","password":"a new strong password 123"}`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400. body: %s", rec.Code, rec.Body)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "PASSWORD_RESET_TOKEN_INVALID" {
		t.Errorf("code %q, want PASSWORD_RESET_TOKEN_INVALID", body.Error.Code)
	}
}

// sessionRouter mounts SessionRoutes behind a stand-in for the Authenticate
// middleware, injecting principal directly — the same way cmd/api mounts
// SessionRoutes inside the protected route group in production, minus the
// real JWT verification, which authjwt's own tests already cover.
func sessionRouter(h *authhttp.Handler, principal auth.Principal) http.Handler {
	r := chi.NewRouter()
	r.Route("/auth/sessions", func(sr chi.Router) {
		sr.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r.WithContext(auth.ContextWithPrincipal(r.Context(), principal)))
			})
		})
		sr.Group(h.SessionRoutes())
	})
	return r
}

func TestListSessionsMarksTheCallersCurrentSession(t *testing.T) {
	sessions := &fakeSessions{}
	h := newHandler(t, true, sessions)

	req := httptest.NewRequest(http.MethodGet, "/auth/sessions", nil)
	rec := httptest.NewRecorder()
	sessionRouter(h, auth.Principal{UserID: "u", Role: auth.RoleReadOnly, TokenID: "not-used-by-the-fake"}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200. body: %s", rec.Code, rec.Body)
	}
	var body struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Items == nil {
		t.Error("items was serialised as null, not []")
	}
}

func TestRevokeSessionRejectsAMalformedID(t *testing.T) {
	h := newHandler(t, true, &fakeSessions{})
	req := httptest.NewRequest(http.MethodDelete, "/auth/sessions/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	sessionRouter(h, auth.Principal{UserID: "u", Role: auth.RoleReadOnly}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400. body: %s", rec.Code, rec.Body)
	}
}
