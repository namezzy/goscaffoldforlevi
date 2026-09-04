// Package ratelimithttp mounts the rate limiter as HTTP middleware.
//
// It is split into three pieces rather than one because the identity a request
// is limited by is only known at different points in the chain: the IP is known
// immediately, the user only after authentication, and the login identifier
// only after the body has been read.
package ratelimithttp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/httpx"
	"github.com/namezzy/goscaffoldforlevi/internal/ratelimit"
	"github.com/namezzy/goscaffoldforlevi/internal/validate"
)

// ByIP limits unauthenticated traffic.
//
// It is mounted before authentication so that a flood never reaches JWT
// verification, which costs a signature check per request.
func ByIP(g *ratelimit.Guard, trustedProxies int, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity := httpx.ClientIPString(r, trustedProxies)
			decision, err := g.Allow(r.Context(), ratelimit.ScopeIP, identity)
			if !proceed(w, r, log, decision, err, true) {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ByUser limits authenticated traffic.
//
// It is mounted after authentication because it needs the principal. Requests
// that somehow reach it unauthenticated are passed through: the IP limiter has
// already seen them, and failing here would be an authorization decision made
// in the wrong place.
func ByUser(g *ratelimit.Guard, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := auth.PrincipalFrom(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			decision, err := g.Allow(r.Context(), ratelimit.ScopeUser, principal.UserID)
			if !proceed(w, r, log, decision, err, true) {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// maxLoginBody bounds how much of the request body the login limiter will read
// in order to find the email. The body has already passed the global body
// limit; this is a second, much tighter bound so that the limiter itself cannot
// be used to make the server buffer megabytes per attempt.
const maxLoginBody = 4 << 10

// Login applies both login scopes.
//
// Two limiters are needed, not one. Limiting by IP alone does not stop a
// botnet spreading attempts for one account across thousands of addresses;
// limiting by account alone lets an attacker deny service to a chosen user by
// deliberately failing their logins. Requiring both closes each other's gap.
//
// Locking the account instead of limiting it was rejected for the same reason:
// a lock needs human intervention to clear, so it can be weaponised into a
// denial of service, while a bucket refills on its own.
func Login(g *ratelimit.Guard, trustedProxies int, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The IP limiter upstream has already advertised its own quota on
			// this response. Those numbers describe general traffic, not login
			// attempts, so they do not leak the attempt counter — but leaving
			// them produces a 429 that also says "199 remaining", which is
			// self-contradictory to anyone reading it. Clearing them here is
			// what makes "login responses carry only Retry-After" true.
			clearRateLimitHeaders(w)

			// The body has to be read here to learn which account is being
			// targeted, and then handed back intact so the handler can decode
			// it normally.
			body, err := io.ReadAll(io.LimitReader(r.Body, maxLoginBody))
			if err != nil {
				httpx.WriteError(r.Context(), w, log,
					errBadLoginBody())
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			ip := httpx.ClientIPString(r, trustedProxies)
			decision, err := g.Allow(r.Context(), ratelimit.ScopeLoginIP, ip)
			if !proceed(w, r, log, decision, err, false) {
				return
			}

			if identity := loginIdentity(body); identity != "" {
				decision, err = g.Allow(r.Context(), ratelimit.ScopeLoginID, identity)
				if !proceed(w, r, log, decision, err, false) {
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// loginIdentity derives the per-account bucket key from the request body.
//
// The email is hashed rather than used directly. Redis keys surface in MONITOR
// output, slow logs and RDB snapshots, and an email address is personal data;
// a digest limits the same account to the same bucket without putting the
// address anywhere it would be read.
func loginIdentity(body []byte) string {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}
	email := validate.NormalizeEmail(req.Email)
	if email == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(email))
	return hex.EncodeToString(sum[:])
}

func errBadLoginBody() error {
	return apperr.Invalid("INVALID_BODY", "unable to read request body")
}

// proceed applies a decision to the response.
//
// exposeHeaders controls whether the X-RateLimit-* headers are set. They are
// useful to legitimate clients, but on the login path they would hand an
// attacker a precise counter of how many attempts remain — so login gets only
// Retry-After.
func proceed(
	w http.ResponseWriter,
	r *http.Request,
	log *slog.Logger,
	decision ratelimit.Decision,
	err error,
	exposeHeaders bool,
) bool {
	if err != nil {
		if errors.Is(err, ratelimit.ErrBackendUnavailable) {
			// A fail-closed scope could not reach its backend. 503 is written
			// directly rather than through an apperr Kind, because it
			// describes a dependency's state and not a business outcome
			// (ADR-0003).
			w.Header().Set("Retry-After", "5")
			http.Error(w, `{"error":{"code":"SERVICE_UNAVAILABLE","message":"service temporarily unavailable"}}`,
				http.StatusServiceUnavailable)
			return false
		}
		httpx.WriteError(r.Context(), w, log, err)
		return false
	}

	if exposeHeaders {
		setRateLimitHeaders(w, decision)
	}
	if decision.Allowed {
		return true
	}

	w.Header().Set("Retry-After", strconv.Itoa(int(decision.RetryAfter/time.Second)))
	httpx.WriteError(r.Context(), w, log, ratelimit.ErrRateLimited())
	return false
}

// clearRateLimitHeaders removes quota advertisements set further up the chain.
func clearRateLimitHeaders(w http.ResponseWriter) {
	for _, h := range [...]string{"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"} {
		w.Header().Del(h)
	}
}

// setRateLimitHeaders advertises the quota on every response, not only on
// rejections, so a client can back off before it is refused.
func setRateLimitHeaders(w http.ResponseWriter, d ratelimit.Decision) {
	if d.Limit <= 0 {
		return
	}
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(d.Limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(max(d.Remaining, 0)))
	if !d.ResetAt.IsZero() {
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(d.ResetAt.Unix(), 10))
	}
}
