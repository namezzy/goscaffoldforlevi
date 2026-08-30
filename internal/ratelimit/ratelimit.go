// Package ratelimit implements token-bucket rate limiting.
//
// Rate limiting is not an optional hardening measure in this service. Argon2id
// spends 64 MiB per login attempt, so an unthrottled login endpoint is a
// memory-exhaustion denial of service with no exploit required (ADR-0008).
package ratelimit

import (
	"context"
	"math"
	"time"
)

// Scope names what is being limited. It is part of the storage key, so two
// scopes never share a bucket even when they use the same identity value.
type Scope string

const (
	// ScopeIP is the blanket limit on unauthenticated traffic.
	ScopeIP Scope = "ip"
	// ScopeUser is the per-account limit applied after authentication, where a
	// higher quota is safe because the caller is identified.
	ScopeUser Scope = "user"
	// ScopeLoginIP resists distributed credential stuffing from one source.
	ScopeLoginIP Scope = "login_ip"
	// ScopeLoginID resists brute force against a single account.
	ScopeLoginID Scope = "login_id"
)

// Rule is a quota expressed the way an operator thinks about it.
type Rule struct {
	// Quota is the sustained number of requests allowed per Period.
	Quota int
	// Period is the window Quota refers to.
	Period time.Duration
	// Burst is the bucket depth: how many requests may arrive at once after an
	// idle stretch. Setting it equal to Quota gives a plain sliding limit;
	// setting it higher tolerates bursty clients without raising the average.
	Burst int
}

// ratePerSecond is the refill rate. It is a float because the interesting
// limits are fractional: five attempts per fifteen minutes is 0.0056 per
// second, and rounding that to an integer would either disable the limit or
// make it absolute.
func (r Rule) ratePerSecond() float64 {
	if r.Period <= 0 || r.Quota <= 0 {
		return 0
	}
	return float64(r.Quota) / r.Period.Seconds()
}

// Valid reports whether the rule can be enforced.
func (r Rule) Valid() bool {
	return r.Quota > 0 && r.Period > 0 && r.Burst > 0
}

// ttl is how long a full bucket takes to become indistinguishable from an
// absent one. Expiring keys at that point stops Redis from accumulating a key
// per IP address forever.
func (r Rule) ttl() time.Duration {
	rate := r.ratePerSecond()
	if rate <= 0 {
		return time.Minute
	}
	return time.Duration(math.Ceil(float64(r.Burst)/rate))*time.Second + time.Second
}

// Decision is the outcome of one Allow call.
type Decision struct {
	Allowed bool
	// Limit and Remaining populate the X-RateLimit-* headers so a well-behaved
	// client can slow down before it is rejected.
	Limit     int
	Remaining int
	// RetryAfter is how long until one more request would be permitted. Token
	// buckets can compute this exactly, which is the main reason the algorithm
	// was chosen: a client that is told 37 seconds does not have to guess.
	RetryAfter time.Duration
	ResetAt    time.Time
}

// Limiter decides whether one request may proceed.
//
// An error means the decision could not be made — not that the request was
// denied. Separating those two outcomes is what lets the caller apply a
// different failure policy per scope.
type Limiter interface {
	Allow(ctx context.Context, scope Scope, identity string, rule Rule) (Decision, error)
}

// allowDecision builds the Decision for a permitted request.
func allowDecision(rule Rule, tokens float64, now time.Time) Decision {
	return Decision{
		Allowed:   true,
		Limit:     rule.Quota,
		Remaining: int(math.Floor(tokens)),
		ResetAt:   now.Add(rule.ttl()),
	}
}

// denyDecision builds the Decision for a rejected request.
func denyDecision(rule Rule, tokens float64, now time.Time) Decision {
	rate := rule.ratePerSecond()
	var retry time.Duration
	if rate > 0 {
		retry = time.Duration(math.Ceil((1 - tokens) / rate * float64(time.Second)))
	}
	if retry < time.Second {
		// Never advertise a sub-second retry: a client that immediately
		// retries produces exactly the load the limit exists to prevent.
		retry = time.Second
	}
	return Decision{
		Allowed:    false,
		Limit:      rule.Quota,
		Remaining:  0,
		RetryAfter: retry,
		ResetAt:    now.Add(retry),
	}
}
