package ratelimit_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/ratelimit"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// stubLimiter stands in for Redis. Making the backend failure injectable is the
// point: the fail-open and fail-closed paths are the decisions that matter most
// and they are unreachable with a healthy backend.
type stubLimiter struct {
	err   error
	calls int
	allow bool
}

func (s *stubLimiter) Allow(_ context.Context, _ ratelimit.Scope, _ string, rule ratelimit.Rule) (ratelimit.Decision, error) {
	s.calls++
	if s.err != nil {
		return ratelimit.Decision{}, s.err
	}
	if s.allow {
		return ratelimit.Decision{Allowed: true, Limit: rule.Quota, Remaining: rule.Burst - 1}, nil
	}
	return ratelimit.Decision{Allowed: false, Limit: rule.Quota, RetryAfter: 30 * time.Second}, nil
}

func testConfig() ratelimit.Config {
	return ratelimit.Config{
		Enabled:  true,
		IP:       ratelimit.Rule{Quota: 3, Period: time.Minute, Burst: 3},
		User:     ratelimit.Rule{Quota: 5, Period: time.Minute, Burst: 5},
		LoginIP:  ratelimit.Rule{Quota: 2, Period: 15 * time.Minute, Burst: 2},
		LoginID:  ratelimit.Rule{Quota: 2, Period: 15 * time.Minute, Burst: 2},
		FailMode: ratelimit.DefaultFailModes(),
	}
}

func TestLocalLimiterAllowsUpToBurstThenDenies(t *testing.T) {
	t.Parallel()

	l := ratelimit.NewLocalLimiter(100)
	rule := ratelimit.Rule{Quota: 3, Period: time.Minute, Burst: 3}

	for i := range 3 {
		d, err := l.Allow(context.Background(), ratelimit.ScopeIP, "1.2.3.4", rule)
		if err != nil {
			t.Fatalf("Allow() = %v", err)
		}
		if !d.Allowed {
			t.Fatalf("request %d was denied while the bucket still had tokens", i+1)
		}
	}

	d, err := l.Allow(context.Background(), ratelimit.ScopeIP, "1.2.3.4", rule)
	if err != nil {
		t.Fatalf("Allow() = %v", err)
	}
	if d.Allowed {
		t.Fatal("the fourth request was allowed past a burst of 3")
	}
	// A client that is refused needs to know how long to wait, or it will
	// hot-loop and produce exactly the load being limited.
	if d.RetryAfter < time.Second {
		t.Errorf("RetryAfter = %v, want at least a second", d.RetryAfter)
	}
}

func TestLocalLimiterRefillsOverTime(t *testing.T) {
	t.Parallel()

	l := ratelimit.NewLocalLimiter(100)
	// One token per second, so the refill is observable without a long test.
	rule := ratelimit.Rule{Quota: 60, Period: time.Minute, Burst: 1}

	if d, _ := l.Allow(context.Background(), ratelimit.ScopeIP, "1.2.3.4", rule); !d.Allowed {
		t.Fatal("the first request was denied")
	}
	if d, _ := l.Allow(context.Background(), ratelimit.ScopeIP, "1.2.3.4", rule); d.Allowed {
		t.Fatal("the second immediate request was allowed")
	}

	// Sleeping is acceptable here because the window is one second and the
	// alternative — injecting a clock into every limiter — would test the
	// injection rather than the refill.
	time.Sleep(1100 * time.Millisecond)

	if d, _ := l.Allow(context.Background(), ratelimit.ScopeIP, "1.2.3.4", rule); !d.Allowed {
		t.Error("the bucket did not refill after the period elapsed")
	}
}

func TestLocalLimiterKeepsScopesAndIdentitiesSeparate(t *testing.T) {
	t.Parallel()

	l := ratelimit.NewLocalLimiter(100)
	rule := ratelimit.Rule{Quota: 1, Period: time.Minute, Burst: 1}

	if d, _ := l.Allow(context.Background(), ratelimit.ScopeIP, "1.2.3.4", rule); !d.Allowed {
		t.Fatal("the first request was denied")
	}
	// A different identity must have its own bucket, or one heavy client would
	// lock out everyone else.
	if d, _ := l.Allow(context.Background(), ratelimit.ScopeIP, "5.6.7.8", rule); !d.Allowed {
		t.Error("a different identity shared the first one's bucket")
	}
	// So must a different scope, or the login limit would be spent by ordinary
	// traffic from the same address.
	if d, _ := l.Allow(context.Background(), ratelimit.ScopeLoginIP, "1.2.3.4", rule); !d.Allowed {
		t.Error("a different scope shared the first one's bucket")
	}
}

func TestLocalLimiterIsBounded(t *testing.T) {
	t.Parallel()

	// The identity comes from the client, so an unbounded map would let an
	// attacker rotating addresses exhaust memory through the very component
	// meant to prevent that.
	const maxEntries = 10
	l := ratelimit.NewLocalLimiter(maxEntries)
	rule := ratelimit.Rule{Quota: 1, Period: time.Hour, Burst: 1}

	for i := range 100 {
		identity := string(rune('a'+i%26)) + string(rune('a'+i/26))
		_, _ = l.Allow(context.Background(), ratelimit.ScopeIP, identity, rule)
	}

	if l.Len() > maxEntries {
		t.Errorf("Len() = %d, want at most %d", l.Len(), maxEntries)
	}
}

func TestGuardFailsOpenForGeneralTraffic(t *testing.T) {
	t.Parallel()

	backend := &stubLimiter{err: errors.New("connection refused")}
	g := ratelimit.NewGuard(testConfig(), backend, discardLogger())

	var observed []ratelimit.Scope
	g.Observer = func(s ratelimit.Scope) { observed = append(observed, s) }

	// Rate limiting protects the service; it is not the service. Turning a
	// Redis outage into a site outage trades a real failure for a hypothetical
	// one.
	d, err := g.Allow(context.Background(), ratelimit.ScopeIP, "1.2.3.4")
	if err != nil {
		t.Fatalf("Allow() = %v, want the request to be served", err)
	}
	if !d.Allowed {
		t.Error("a general request was denied because the backend was down")
	}
	if len(observed) != 1 || observed[0] != ratelimit.ScopeIP {
		t.Errorf("observed = %v, want one ip-scope backend error", observed)
	}
}

func TestGuardFallsBackToTheLocalLimiterWhenTheBackendIsDown(t *testing.T) {
	t.Parallel()

	backend := &stubLimiter{err: errors.New("connection refused")}
	cfg := testConfig()
	cfg.IP = ratelimit.Rule{Quota: 1, Period: time.Hour, Burst: 1}
	g := ratelimit.NewGuard(cfg, backend, discardLogger())

	// Failing open must not mean "no limit at all": the in-process limiter still
	// bounds a flood, at a deliberately looser quota.
	allowed := 0
	for range 20 {
		if d, err := g.Allow(context.Background(), ratelimit.ScopeIP, "1.2.3.4"); err == nil && d.Allowed {
			allowed++
		}
	}
	if allowed == 0 {
		t.Fatal("the fallback denied everything; that is fail-closed behaviour")
	}
	if allowed == 20 {
		t.Error("the fallback allowed every request; there is no backstop at all")
	}
}

func TestGuardFailsClosedForLogin(t *testing.T) {
	t.Parallel()

	backend := &stubLimiter{err: errors.New("connection refused")}
	g := ratelimit.NewGuard(testConfig(), backend, discardLogger())

	// Allowing unlimited login attempts while the limiter is blind removes
	// brute-force protection at the moment it cannot be observed, and leaves
	// the 64 MiB-per-attempt Argon2 cost unbounded.
	for _, scope := range []ratelimit.Scope{ratelimit.ScopeLoginIP, ratelimit.ScopeLoginID} {
		_, err := g.Allow(context.Background(), scope, "1.2.3.4")
		if !errors.Is(err, ratelimit.ErrBackendUnavailable) {
			t.Errorf("Allow(%s) = %v, want ErrBackendUnavailable", scope, err)
		}
	}
}

func TestGuardHonoursTheBackendDecision(t *testing.T) {
	t.Parallel()

	backend := &stubLimiter{allow: false}
	g := ratelimit.NewGuard(testConfig(), backend, discardLogger())

	d, err := g.Allow(context.Background(), ratelimit.ScopeUser, "user-1")
	if err != nil {
		t.Fatalf("Allow() = %v", err)
	}
	if d.Allowed {
		t.Error("Allow() overrode a rejection from the backend")
	}
	if backend.calls != 1 {
		t.Errorf("backend calls = %d, want 1", backend.calls)
	}
}

func TestGuardDisabledAllowsEverythingWithoutTouchingTheBackend(t *testing.T) {
	t.Parallel()

	backend := &stubLimiter{allow: false}
	cfg := testConfig()
	cfg.Enabled = false
	g := ratelimit.NewGuard(cfg, backend, discardLogger())

	d, err := g.Allow(context.Background(), ratelimit.ScopeIP, "1.2.3.4")
	if err != nil || !d.Allowed {
		t.Fatalf("Allow() = (%+v, %v), want an allow", d, err)
	}
	if backend.calls != 0 {
		t.Errorf("backend calls = %d, want 0 when limiting is disabled", backend.calls)
	}
}

func TestGuardWithoutABackendUsesTheLocalLimiter(t *testing.T) {
	t.Parallel()

	// A nil primary is the state the composition root produces when Redis is
	// configured as optional and is unavailable at startup.
	cfg := testConfig()
	cfg.IP = ratelimit.Rule{Quota: 1, Period: time.Hour, Burst: 1}
	g := ratelimit.NewGuard(cfg, nil, discardLogger())

	allowed := 0
	for range 10 {
		if d, err := g.Allow(context.Background(), ratelimit.ScopeIP, "1.2.3.4"); err == nil && d.Allowed {
			allowed++
		}
	}
	if allowed == 0 || allowed == 10 {
		t.Errorf("allowed %d of 10; want a bounded but non-zero number", allowed)
	}
}

func TestGuardIgnoresAnEmptyIdentity(t *testing.T) {
	t.Parallel()

	backend := &stubLimiter{allow: false}
	g := ratelimit.NewGuard(testConfig(), backend, discardLogger())

	// An empty identity would put every anonymous caller into one bucket, which
	// is a global limit wearing a per-client disguise.
	d, err := g.Allow(context.Background(), ratelimit.ScopeIP, "")
	if err != nil || !d.Allowed {
		t.Fatalf("Allow() = (%+v, %v), want an allow", d, err)
	}
	if backend.calls != 0 {
		t.Errorf("backend calls = %d, want 0", backend.calls)
	}
}

// TestRemainingNeverExceedsLimit covers a defect found by reading the response
// headers of a plain GET: X-RateLimit-Limit was 600 while
// X-RateLimit-Remaining was 999.
//
// Clients derive their usage as limit-remaining, so that pair says the caller
// has made -399 requests. The cause is that the two numbers were counting
// different things: Limit reported the sustained quota while Remaining
// reported the depth of the token bucket, and the default general rule allows
// a burst of 1000 against a quota of 600/min.
//
// The bucket depth is the honest answer to "how many requests may I send right
// now", so Limit follows Remaining rather than the other way round. Where
// burst equals quota — every login rule — nothing changes.
func TestRemainingNeverExceedsLimit(t *testing.T) {
	t.Parallel()

	// Burst deliberately above Quota, which is the configuration that made the
	// inconsistency visible.
	rule := ratelimit.Rule{Quota: 600, Period: time.Minute, Burst: 1000}

	check := func(t *testing.T, what string, d ratelimit.Decision) {
		t.Helper()
		if d.Remaining < 0 || d.Remaining > d.Limit {
			t.Errorf("%s: remaining = %d, limit = %d, want 0 <= remaining <= limit",
				what, d.Remaining, d.Limit)
		}
	}

	t.Run("allowed", func(t *testing.T) {
		t.Parallel()
		limiter := ratelimit.NewLocalLimiter(16)
		d, err := limiter.Allow(context.Background(), ratelimit.ScopeIP, "someone", rule)
		if err != nil {
			t.Fatalf("Allow: %v", err)
		}
		if !d.Allowed {
			t.Fatal("the first request against a fresh bucket was denied")
		}
		check(t, "first request", d)
	})

	t.Run("denied", func(t *testing.T) {
		t.Parallel()
		limiter := ratelimit.NewLocalLimiter(16)
		tight := ratelimit.Rule{Quota: 1, Period: time.Minute, Burst: 1}
		for i := 0; i < 3; i++ {
			d, err := limiter.Allow(context.Background(), ratelimit.ScopeIP, "someone", tight)
			if err != nil {
				t.Fatalf("Allow: %v", err)
			}
			check(t, "request after exhaustion", d)
		}
	})

	t.Run("guard disabled", func(t *testing.T) {
		t.Parallel()
		guard := ratelimit.NewGuard(
			ratelimit.Config{Enabled: false, IP: rule},
			ratelimit.NewLocalLimiter(16),
			discardLogger(),
		)
		d, err := guard.Allow(context.Background(), ratelimit.ScopeIP, "someone")
		if err != nil {
			t.Fatalf("Allow: %v", err)
		}
		check(t, "limiting disabled", d)
	})
}
