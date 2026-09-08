package ratelimithttp_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/ratelimit"
	"github.com/namezzy/goscaffoldforlevi/internal/ratelimit/ratelimithttp"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// rule allows two requests before refusing, which is the smallest budget that
// still distinguishes "allowed" from "always refused".
func rule() ratelimit.Rule {
	return ratelimit.Rule{Quota: 2, Period: time.Minute, Burst: 2}
}

// newGuard wires a working in-process limiter as the primary. Passing nil
// instead would exercise the Guard's degraded fallback, whose budget is
// deliberately different, and these tests are about the middleware rather than
// about the fallback.
func newGuard(t *testing.T, primary ratelimit.Limiter) *ratelimit.Guard {
	t.Helper()

	return ratelimit.NewGuard(ratelimit.Config{
		Enabled:  true,
		IP:       rule(),
		User:     rule(),
		LoginIP:  rule(),
		LoginID:  rule(),
		FailMode: ratelimit.DefaultFailModes(),
	}, primary, discardLogger())
}

// brokenLimiter stands in for an unreachable Redis.
type brokenLimiter struct{}

func (brokenLimiter) Allow(context.Context, ratelimit.Scope, string, ratelimit.Rule) (ratelimit.Decision, error) {
	return ratelimit.Decision{}, errors.New("dial tcp: connection refused")
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestByIPRefusesOnceTheBudgetIsSpent(t *testing.T) {
	t.Parallel()

	h := ratelimithttp.ByIP(newGuard(t, ratelimit.NewLocalLimiter(1000)), 0, discardLogger())(okHandler())

	send := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	for i := 1; i <= 2; i++ {
		if rec := send(); rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, rec.Code)
		}
	}

	rec := send()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429: %s", rec.Code, rec.Body)
	}

	// Retry-After is what makes a 429 actionable; without it a client can only
	// guess, and guessing means retrying immediately.
	retry, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || retry < 1 {
		t.Errorf("Retry-After = %q, want a positive integer", rec.Header().Get("Retry-After"))
	}
	if got := rec.Header().Get("X-RateLimit-Remaining"); got != "0" {
		t.Errorf("X-RateLimit-Remaining = %q, want 0", got)
	}
	if got := rec.Header().Get("X-RateLimit-Limit"); got != "2" {
		t.Errorf("X-RateLimit-Limit = %q, want 2", got)
	}
}

// TestQuotaIsAdvertisedBeforeItRunsOut matters because headers sent only on
// rejection arrive too late to be useful.
func TestQuotaIsAdvertisedBeforeItRunsOut(t *testing.T) {
	t.Parallel()

	h := ratelimithttp.ByIP(newGuard(t, ratelimit.NewLocalLimiter(1000)), 0, discardLogger())(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.11:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("X-RateLimit-Limit") == "" || rec.Header().Get("X-RateLimit-Reset") == "" {
		t.Errorf("a successful response should carry the quota headers: %v", rec.Header())
	}
}

// TestByIPBucketsPerAddress guards against a single global counter, which would
// let one busy client refuse service to everyone else.
func TestByIPBucketsPerAddress(t *testing.T) {
	t.Parallel()

	h := ratelimithttp.ByIP(newGuard(t, ratelimit.NewLocalLimiter(1000)), 0, discardLogger())(okHandler())

	send := func(addr string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = addr + ":1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	for i := 0; i < 3; i++ {
		send("203.0.113.20")
	}
	if code := send("203.0.113.21"); code != http.StatusOK {
		t.Errorf("a second address was refused: status = %d", code)
	}
}

func TestByUserBucketsPerPrincipalAndSkipsAnonymousRequests(t *testing.T) {
	t.Parallel()

	h := ratelimithttp.ByUser(newGuard(t, ratelimit.NewLocalLimiter(1000)), discardLogger())(okHandler())

	send := func(userID string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if userID != "" {
			req = req.WithContext(auth.ContextWithPrincipal(req.Context(),
				auth.Principal{UserID: userID, Role: auth.RoleUser}))
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	for i := 0; i < 3; i++ {
		send("user-a")
	}
	if code := send("user-b"); code != http.StatusOK {
		t.Errorf("a second user was refused: status = %d", code)
	}
	// Without a principal there is nothing to bucket by; the IP limiter has
	// already seen the request, so refusing here would be a decision made in
	// the wrong place.
	for i := 0; i < 5; i++ {
		if code := send(""); code != http.StatusOK {
			t.Fatalf("anonymous request refused: status = %d", code)
		}
	}
}

// TestLoginHidesTheRemainingAttemptCount is the difference between the login
// limiter and the others: the same headers that help a well-behaved client tell
// an attacker exactly how many guesses are left before the block.
func TestLoginHidesTheRemainingAttemptCount(t *testing.T) {
	t.Parallel()

	guard := newGuard(t, ratelimit.NewLocalLimiter(1000))
	// The IP limiter is mounted above login in the real router and sets its own
	// headers, so the login middleware has to clear them rather than merely
	// decline to add any.
	h := ratelimithttp.ByIP(guard, 0, discardLogger())(
		ratelimithttp.Login(guard, 0, discardLogger())(okHandler()))

	send := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/auth/login",
			strings.NewReader(`{"email":"victim@example.com","password":"x"}`))
		req.RemoteAddr = "203.0.113.30:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	rec := send()
	if rec.Code != http.StatusOK {
		t.Fatalf("first attempt: status = %d", rec.Code)
	}
	for _, header := range []string{"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"} {
		if rec.Header().Get(header) != "" {
			t.Errorf("login response exposes %s", header)
		}
	}

	var last *httptest.ResponseRecorder
	for i := 0; i < 3; i++ {
		last = send()
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", last.Code)
	}
	// Retry-After is still sent: it is a fixed property of the rule, not a
	// count of what the attacker has left.
	if last.Header().Get("Retry-After") == "" {
		t.Error("a login 429 should still carry Retry-After")
	}
}

// TestLoginLimitsPerAccountAcrossAddresses covers the case IP limiting alone
// misses: a botnet attacking one account from many addresses.
func TestLoginLimitsPerAccountAcrossAddresses(t *testing.T) {
	t.Parallel()

	h := ratelimithttp.Login(newGuard(t, ratelimit.NewLocalLimiter(1000)), 0, discardLogger())(okHandler())

	send := func(addr, email string) int {
		req := httptest.NewRequest(http.MethodPost, "/auth/login",
			strings.NewReader(`{"email":"`+email+`","password":"x"}`))
		req.RemoteAddr = addr + ":1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	for i := 0; i < 2; i++ {
		if code := send("198.51.100."+strconv.Itoa(i), "victim@example.com"); code != http.StatusOK {
			t.Fatalf("attempt %d refused early: status = %d", i, code)
		}
	}
	if code := send("198.51.100.99", "victim@example.com"); code != http.StatusTooManyRequests {
		t.Errorf("a fresh address bypassed the per-account limit: status = %d", code)
	}
	// The address is what changed, so a different account from a new address
	// must still be served.
	if code := send("198.51.100.98", "other@example.com"); code != http.StatusOK {
		t.Errorf("an unrelated account was caught by another account's limit: status = %d", code)
	}
}

// TestEmailCasingSharesOneBucket: "Victim@..." and "victim@..." are the same
// account, so treating them as different buckets would double the budget.
func TestEmailCasingSharesOneBucket(t *testing.T) {
	t.Parallel()

	h := ratelimithttp.Login(newGuard(t, ratelimit.NewLocalLimiter(1000)), 0, discardLogger())(okHandler())

	send := func(email string) int {
		req := httptest.NewRequest(http.MethodPost, "/auth/login",
			strings.NewReader(`{"email":"`+email+`","password":"x"}`))
		req.RemoteAddr = "198.51.100.5:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	send("victim@example.com")
	send("  VICTIM@Example.COM ")
	if code := send("Victim@example.com"); code != http.StatusTooManyRequests {
		t.Errorf("case variants were counted separately: status = %d", code)
	}
}

func TestLoginRestoresTheBodyForTheHandler(t *testing.T) {
	t.Parallel()

	const body = `{"email":"user@example.com","password":"secret"}`

	var seen string
	h := ratelimithttp.Login(newGuard(t, ratelimit.NewLocalLimiter(1000)), 0, discardLogger())(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			seen = string(raw)
			w.WriteHeader(http.StatusOK)
		}))

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.RemoteAddr = "198.51.100.6:1234"
	h.ServeHTTP(httptest.NewRecorder(), req)

	// The middleware must consume the body to find the account, so it has to
	// put it back; otherwise the handler sees an empty request.
	if seen != body {
		t.Errorf("handler read %q, want the original body", seen)
	}
}

// TestBackendFailureFailsOpenForTrafficAndClosedForLogin is the pair of
// decisions from ADR-0008, observed at the HTTP boundary where they matter.
func TestBackendFailureFailsOpenForTrafficAndClosedForLogin(t *testing.T) {
	t.Parallel()

	guard := newGuard(t, brokenLimiter{})

	// General traffic: rate limiting is a protection, not a feature. Refusing
	// everything because Redis is down converts a hiccup into an outage.
	byIP := ratelimithttp.ByIP(guard, 0, discardLogger())(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.40:1234"
	rec := httptest.NewRecorder()
	byIP.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("general traffic: status = %d, want 200", rec.Code)
	}

	// Login: allowing unlimited attempts while blind removes brute-force
	// protection exactly when nobody can see it happening.
	login := ratelimithttp.Login(guard, 0, discardLogger())(okHandler())
	req = httptest.NewRequest(http.MethodPost, "/auth/login",
		strings.NewReader(`{"email":"victim@example.com","password":"x"}`))
	req.RemoteAddr = "203.0.113.41:1234"
	rec = httptest.NewRecorder()
	login.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("login: status = %d, want 503", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("the 503 should tell the client when to come back")
	}
}
