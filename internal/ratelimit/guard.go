package ratelimit

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
)

// FailMode is what to do when the limiter backend cannot answer.
//
// There is no single correct choice, which is why it is a per-scope setting.
type FailMode int

const (
	// FailOpen lets the request through. Correct for general traffic limits:
	// rate limiting is a protection, not a feature, and taking the whole site
	// down because Redis hiccuped trades a real outage for a hypothetical one.
	FailOpen FailMode = iota
	// FailClosed rejects the request. Correct for login: allowing unlimited
	// attempts while the backend is down disables brute-force protection at
	// exactly the moment it cannot be observed, and leaves the 64 MiB-per-try
	// Argon2id cost unbounded. A brief inability to log in is the smaller harm.
	FailClosed
)

// Config is the full limiting policy.
type Config struct {
	Enabled  bool
	IP       Rule
	User     Rule
	LoginIP  Rule
	LoginID  Rule
	FailMode map[Scope]FailMode
}

// DefaultFailModes encodes the split described in ADR-0008.
func DefaultFailModes() map[Scope]FailMode {
	return map[Scope]FailMode{
		ScopeIP:      FailOpen,
		ScopeUser:    FailOpen,
		ScopeLoginIP: FailClosed,
		ScopeLoginID: FailClosed,
	}
}

// Rule returns the rule configured for a scope.
func (c Config) Rule(s Scope) Rule {
	switch s {
	case ScopeIP:
		return c.IP
	case ScopeUser:
		return c.User
	case ScopeLoginIP:
		return c.LoginIP
	case ScopeLoginID:
		return c.LoginID
	default:
		return Rule{}
	}
}

// Guard applies the policy on top of a primary limiter.
//
// It owns three things the raw Limiter deliberately does not: the fallback to
// the in-process limiter, the per-scope failure mode, and the suppression of
// log floods.
type Guard struct {
	cfg      Config
	primary  Limiter
	fallback *LocalLimiter
	log      *slog.Logger

	sampler *logSampler
	// Observer is called for every backend error so Phase 5 can attach a
	// counter without this package importing a metrics library.
	Observer func(scope Scope)

	// RejectionObserver is called for every rejected request, so Phase 5 can
	// attach a counter without this package importing a metrics library.
	//
	// It is deliberately outside the log sampler. The log is sampled because
	// an attack would otherwise make the defence generate a flood of its own;
	// a counter costs nothing per event, and sampling it would report one
	// rejection per interval whether the attacker sent two requests or two
	// million — destroying the only number that says how big the attack is.
	RejectionObserver func(scope Scope)
}

// fallbackFactor loosens the in-process quota relative to the real one. The
// local limiter is per-replica, so applying the configured quota there would
// multiply the real limit by the replica count anyway; making it deliberately
// generous states that it is a stampede guard, not the policy.
const fallbackFactor = 2

// NewGuard builds a Guard. primary may be nil, in which case every request is
// served by the in-process fallback — the intended behaviour when Redis is not
// configured at all.
func NewGuard(cfg Config, primary Limiter, log *slog.Logger) *Guard {
	if cfg.FailMode == nil {
		cfg.FailMode = DefaultFailModes()
	}
	return &Guard{
		cfg:      cfg,
		primary:  primary,
		fallback: NewLocalLimiter(50000),
		log:      log,
		sampler:  newLogSampler(10 * time.Second),
	}
}

// ErrBackendUnavailable is returned when a fail-closed scope cannot reach its
// backend.
//
// It is a sentinel rather than an apperr Kind because 503 describes the state
// of a dependency, not a classification of a business error; ADR-0003 keeps
// such codes out of the Kind set and has the transport write them directly.
// The caller therefore checks for this error and answers 503 itself.
var ErrBackendUnavailable = errors.New("ratelimit: backend unavailable")

// ErrRateLimited is the rejection returned to the client.
func ErrRateLimited() error {
	return apperr.RateLimited("RATE_LIMITED", "too many requests")
}

// Allow evaluates one scope.
//
// The returned Decision is always populated, even on the fail-open path, so the
// caller can set X-RateLimit-* headers without special-casing.
func (g *Guard) Allow(ctx context.Context, scope Scope, identity string) (Decision, error) {
	rule := g.cfg.Rule(scope)
	if !g.cfg.Enabled || !rule.Valid() || identity == "" {
		return Decision{Allowed: true, Limit: rule.Burst, Remaining: rule.Burst}, nil
	}

	if g.primary != nil {
		decision, err := g.primary.Allow(ctx, scope, identity, rule)
		if err == nil {
			g.logIfDenied(ctx, scope, identity, decision)
			return decision, nil
		}
		g.onBackendError(ctx, scope, err)

		if g.cfg.FailMode[scope] == FailClosed {
			return Decision{}, ErrBackendUnavailable
		}
	}

	// Fail-open path, or no primary configured at all.
	loose := Rule{Quota: rule.Quota * fallbackFactor, Period: rule.Period, Burst: rule.Burst * fallbackFactor}
	decision, _ := g.fallback.Allow(ctx, scope, identity, loose)
	g.logIfDenied(ctx, scope, identity, decision)
	return decision, nil
}

func (g *Guard) onBackendError(ctx context.Context, scope Scope, err error) {
	if g.Observer != nil {
		g.Observer(scope)
	}
	// The identity is omitted: it is an IP address or an email digest, and a
	// backend failure is not a reason to start writing those into logs.
	g.log.WarnContext(ctx, "rate limiter backend unavailable",
		slog.String("scope", string(scope)),
		slog.String("error", err.Error()),
	)
}

// logIfDenied records a rejection, sampled per identity.
//
// Unsampled, this is a log amplifier: an attack produces one line per attempt,
// so the defence against the flood generates a flood of its own. Metrics stay
// unsampled because a counter costs nothing per event.
func (g *Guard) logIfDenied(ctx context.Context, scope Scope, identity string, d Decision) {
	if d.Allowed {
		return
	}
	if g.RejectionObserver != nil {
		g.RejectionObserver(scope)
	}
	if !g.sampler.allow(string(scope) + ":" + identity) {
		return
	}
	g.log.WarnContext(ctx, "rate limit exceeded",
		slog.String("scope", string(scope)),
		slog.Duration("retry_after", d.RetryAfter),
	)
}

// logSampler admits one event per key per interval.
type logSampler struct {
	mu       sync.Mutex
	interval time.Duration
	last     map[string]time.Time
	now      func() time.Time
}

func newLogSampler(interval time.Duration) *logSampler {
	return &logSampler{interval: interval, last: make(map[string]time.Time), now: time.Now}
}

// maxSamplerEntries bounds the sampler's own memory, for the same reason the
// local limiter is bounded: its keys come from the client.
const maxSamplerEntries = 10000

func (s *logSampler) allow(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	if last, ok := s.last[key]; ok && now.Sub(last) < s.interval {
		return false
	}
	if len(s.last) >= maxSamplerEntries {
		for k, t := range s.last {
			if now.Sub(t) >= s.interval {
				delete(s.last, k)
			}
		}
		if len(s.last) >= maxSamplerEntries {
			// Full of recent entries: drop the log line rather than the bound.
			return false
		}
	}
	s.last[key] = now
	return true
}
