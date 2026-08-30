package ratelimit

import (
	"context"
	"sync"
	"time"
)

// LocalLimiter is an in-process token bucket.
//
// It is not a substitute for the Redis limiter — with N replicas the effective
// quota is N times the configured one, and it resets on restart. It exists as
// the backstop for when Redis is unavailable: without it, a Redis outage would
// remove rate limiting entirely for scopes that fail open, which is precisely
// when a service is least able to absorb a flood.
//
// Its quotas are therefore set deliberately loose (see NewGuard): the goal is
// to prevent a stampede, not to enforce the real policy.
type LocalLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	now     func() time.Time
	// maxEntries bounds memory. The key includes a client-controlled identity,
	// so an attacker rotating IPs or emails would otherwise grow this map
	// without limit — turning the DoS defence into a DoS.
	maxEntries int
}

type bucket struct {
	tokens   float64
	last     time.Time
	lastSeen time.Time
}

// NewLocalLimiter builds an in-process limiter holding at most maxEntries
// buckets.
func NewLocalLimiter(maxEntries int) *LocalLimiter {
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	return &LocalLimiter{
		buckets:    make(map[string]*bucket),
		now:        time.Now,
		maxEntries: maxEntries,
	}
}

var _ Limiter = (*LocalLimiter)(nil)

// Allow consumes one token. It never returns an error: there is no backend that
// can fail, which is what makes it usable as the fallback.
func (l *LocalLimiter) Allow(_ context.Context, scope Scope, identity string, rule Rule) (Decision, error) {
	if !rule.Valid() {
		return Decision{Allowed: true}, nil
	}

	key := string(scope) + ":" + identity
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= l.maxEntries {
			l.evictStale(now, rule.ttl())
		}
		// If eviction freed nothing, the map is full of live buckets and the
		// process is under a genuine flood. Allowing the request is the wrong
		// answer there, so the new caller starts from an empty bucket and is
		// denied until capacity frees up.
		if len(l.buckets) >= l.maxEntries {
			return denyDecision(rule, 0, now), nil
		}
		b = &bucket{tokens: float64(rule.Burst), last: now}
		l.buckets[key] = b
	}

	elapsed := now.Sub(b.last)
	if elapsed < 0 {
		elapsed = 0
	}
	b.tokens = min(float64(rule.Burst), b.tokens+elapsed.Seconds()*rule.ratePerSecond())
	b.last = now
	b.lastSeen = now

	if b.tokens < 1 {
		return denyDecision(rule, b.tokens, now), nil
	}
	b.tokens--
	return allowDecision(rule, b.tokens, now), nil
}

// evictStale drops buckets that have had time to refill completely. Such a
// bucket is indistinguishable from one that does not exist, so removing it
// loses no information.
func (l *LocalLimiter) evictStale(now time.Time, ttl time.Duration) {
	for k, b := range l.buckets {
		if now.Sub(b.lastSeen) >= ttl {
			delete(l.buckets, k)
		}
	}
}

// Len reports the number of live buckets. It exists for tests and metrics.
func (l *LocalLimiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
