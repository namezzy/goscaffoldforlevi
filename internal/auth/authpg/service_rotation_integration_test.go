//go:build integration

package authpg_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/example/go-enterprise/internal/apperr"
	"github.com/example/go-enterprise/internal/auth"
	"github.com/example/go-enterprise/internal/auth/authhttp"
	"github.com/example/go-enterprise/internal/auth/authjwt"
	"github.com/example/go-enterprise/internal/auth/authpg"
	"github.com/example/go-enterprise/internal/openapitest"
	"github.com/example/go-enterprise/internal/postgres"
	"github.com/example/go-enterprise/internal/postgres/pgtest"
	"github.com/example/go-enterprise/internal/user"
	"github.com/example/go-enterprise/internal/user/userpg"
)

// This file wires the real service to the real database, because the property
// it checks only exists there.
//
// Reuse detection revokes the token family and then rejects the request. Under
// a fake transaction manager that merely calls the function, both happen. Under
// a real one, returning an error rolls the revocation back — the stolen family
// stays live and the response looks correct. The unit tests missed this exact
// bug; it was found by running the service against PostgreSQL.

const rotationPassword = "correct horse battery staple"

func newRotationService(t *testing.T, pg *pgtest.Container) (*auth.Service, user.User) {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Argon2id at production cost would add ~50 ms per call for no benefit
	// here; the parameters are exercised in the unit tests.
	hasher, err := auth.NewArgon2Hasher(auth.Argon2Params{
		Time: 1, Memory: 8 << 10, Threads: 1, SaltLen: 16, KeyLen: 32,
	})
	if err != nil {
		t.Fatalf("hasher: %v", err)
	}
	hash, err := hasher.Hash(rotationPassword)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	users := userpg.New(pg.Pool)
	owner, err := users.Create(context.Background(), user.User{
		ID:           user.NewID(),
		Email:        "rotation@example.com",
		Name:         "Rotation",
		Status:       user.StatusActive,
		Role:         auth.RoleUser,
		PasswordHash: hash,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	tokens, err := authjwt.New("a-test-signing-secret-of-sufficient-length", "test", "test-api")
	if err != nil {
		t.Fatalf("jwt: %v", err)
	}

	svc := auth.NewService(
		auth.ServiceConfig{
			Issuer:     "test",
			Audience:   "test-api",
			AccessTTL:  15 * time.Minute,
			RefreshTTL: 24 * time.Hour,
		},
		user.NewAccountStore(users),
		authpg.NewSessionRepository(pg.Pool),
		hasher,
		tokens,
		postgres.NewTxManager(pg.Pool),
		log,
	)
	return svc, owner
}

func activeSessions(t *testing.T, pg *pgtest.Container, userID string) int {
	t.Helper()

	var n int
	if err := pg.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM refresh_sessions WHERE user_id = $1 AND revoked_at IS NULL`,
		userID,
	).Scan(&n); err != nil {
		t.Fatalf("count active sessions: %v", err)
	}
	return n
}

// TestReplayedRefreshTokenKillsTheFamilyForReal is the regression test for the
// rollback bug: the revocation has to survive the rejection.
func TestReplayedRefreshTokenKillsTheFamilyForReal(t *testing.T) {
	pg := pgtest.Start(t)
	svc, owner := newRotationService(t, pg)
	ctx := context.Background()

	first, err := svc.Login(ctx, "rotation@example.com", rotationPassword, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	second, err := svc.Refresh(ctx, first.RefreshToken, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Fatal("rotation returned the same refresh token")
	}

	// The attacker replays the token the legitimate client already spent.
	if _, err := svc.Refresh(ctx, first.RefreshToken, auth.RequestMeta{}); err == nil {
		t.Fatal("the replayed token was accepted")
	}

	// The response was a rejection either way. What distinguishes a working
	// implementation from a broken one is whether the revocation was committed.
	if n := activeSessions(t, pg, owner.ID.String()); n != 0 {
		t.Errorf("%d sessions are still live after reuse was detected", n)
	}
	if _, err := svc.Refresh(ctx, second.RefreshToken, auth.RequestMeta{}); err == nil {
		t.Error("the legitimate client's current token still works after a replay")
	}
}

// TestDisablingAnAccountTakesEffectAtTheNextRotation covers the other path that
// revokes and then rejects, and it is the reason deactivation does not have to
// wait out the refresh lifetime.
func TestDisablingAnAccountTakesEffectAtTheNextRotation(t *testing.T) {
	pg := pgtest.Start(t)
	svc, owner := newRotationService(t, pg)
	users := userpg.New(pg.Pool)
	ctx := context.Background()

	pair, err := svc.Login(ctx, "rotation@example.com", rotationPassword, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	disabled := user.StatusDisabled
	if _, err := users.Update(ctx, owner.ID, user.Patch{Status: &disabled}); err != nil {
		t.Fatalf("disable: %v", err)
	}

	if _, err := svc.Refresh(ctx, pair.RefreshToken, auth.RequestMeta{}); err == nil {
		t.Fatal("a disabled account rotated its token")
	}
	if n := activeSessions(t, pg, owner.ID.String()); n != 0 {
		t.Errorf("%d sessions survived the account being disabled", n)
	}

	// The account can no longer log in either, and the failure is the same one
	// a wrong password produces.
	_, err = svc.Login(ctx, "rotation@example.com", rotationPassword, auth.RequestMeta{})
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != "INVALID_CREDENTIALS" {
		t.Errorf("login as a disabled account returned %v, want INVALID_CREDENTIALS", err)
	}
}

// TestLogoutRevokesTheFamilyAndIsIdempotent checks the behaviour a client
// relies on when it retries a logout it never saw the response to.
func TestLogoutRevokesTheFamilyAndIsIdempotent(t *testing.T) {
	pg := pgtest.Start(t)
	svc, owner := newRotationService(t, pg)
	ctx := context.Background()

	pair, err := svc.Login(ctx, "rotation@example.com", rotationPassword, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	rotated, err := svc.Refresh(ctx, pair.RefreshToken, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}

	if err := svc.Logout(ctx, rotated.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if n := activeSessions(t, pg, owner.ID.String()); n != 0 {
		t.Errorf("%d sessions survived logout", n)
	}
	if err := svc.Logout(ctx, rotated.RefreshToken); err != nil {
		t.Errorf("a repeated logout failed: %v", err)
	}
	// An unknown token is also a success: reporting "no such session" would let
	// an unauthenticated caller probe which tokens are live.
	if err := svc.Logout(ctx, "a-token-that-never-existed"); err != nil {
		t.Errorf("logout of an unknown token failed: %v", err)
	}
}

// TestTokenResponsesMatchTheDocumentedSchema checks the authentication
// endpoints' bodies against api/openapi.yaml.
//
// It lives here rather than in authhttp because the handler takes the concrete
// service, and a token pair produced by anything other than the real service is
// a token pair whose shape the test authors chose. Login and refresh are both
// exercised: they share a response type today, and this is what would notice if
// one of them stopped doing so.
func TestTokenResponsesMatchTheDocumentedSchema(t *testing.T) {
	pg := pgtest.Start(t)
	svc, _ := newRotationService(t, pg)
	spec := openapitest.Load(t)

	handler := authhttp.NewHandler(svc, 0, slog.New(slog.NewTextHandler(io.Discard, nil)))
	router := chi.NewRouter()
	router.Route("/", handler.Routes(func(next http.Handler) http.Handler { return next }))

	post := func(path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	login := post("/login", `{"email":"rotation@example.com","password":"`+rotationPassword+`"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login: status = %d: %s", login.Code, login.Body)
	}
	spec.AssertMatches(t, "TokenPair", login.Body.Bytes())

	var pair struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &pair); err != nil {
		t.Fatalf("decode the token pair: %v", err)
	}

	refreshed := post("/refresh", `{"refresh_token":"`+pair.RefreshToken+`"}`)
	if refreshed.Code != http.StatusOK {
		t.Fatalf("refresh: status = %d: %s", refreshed.Code, refreshed.Body)
	}
	spec.AssertMatches(t, "TokenPair", refreshed.Body.Bytes())

	rejected := post("/login", `{"email":"rotation@example.com","password":"wrong"}`)
	if rejected.Code != http.StatusUnauthorized {
		t.Fatalf("bad login: status = %d: %s", rejected.Code, rejected.Body)
	}
	spec.AssertMatches(t, "Error", rejected.Body.Bytes())
}
