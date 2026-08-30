package httpx

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/example/go-enterprise/internal/apperr"
	"github.com/example/go-enterprise/internal/config"
)

// chiRouter is the slice of chi.Router used for mounting routes, kept narrow
// so that route registration is easy to fake in tests.
type chiRouter interface {
	Get(pattern string, h http.HandlerFunc)
}

// RouterDeps carries what the router needs. It is a struct rather than a long
// parameter list so that adding a dependency in a later phase does not become
// a rename across every call site.
type RouterDeps struct {
	Config config.Config
	Logger *slog.Logger
	Health *Health

	// Register is called with the /api/v1 subrouter. Feature packages attach
	// their routes here, which keeps httpx free of any business imports.
	Register func(r chi.Router)

	// RateLimitByIP guards unauthenticated traffic. It is injected rather than
	// constructed here so that httpx does not depend on the limiter, and so a
	// test can mount the stack without a Redis instance.
	//
	// A nil value disables it. That is a real configuration — RATELIMIT_ENABLED
	// exists — but it is the composition root's decision to make, not this
	// function's.
	RateLimitByIP func(http.Handler) http.Handler
}

// NewRouter assembles the middleware stack and the route tree.
//
// The order below is load-bearing and is justified in docs/architecture.md §5.
// The parts worth restating:
//
//   - Recovery is outermost so a panic anywhere still produces a response.
//   - RequestID precedes Logging so every line can be correlated.
//   - CORS precedes the rate limiters so a preflight never consumes quota.
//   - Metrics (Phase 5) precede the rate limiters so that 429s are counted.
//   - Timeout precedes the handlers so its deadline reaches the database.
//   - RateLimitByIP precedes authentication, so a flood of forged tokens is
//     rejected before anything verifies a signature.
//
// Authentication, per-user limiting and authorization are mounted by the
// feature packages on the routes that need them, because each depends on
// something this function does not know: the token verifier, the principal, and
// the target record respectively.
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	r.Use(Recovery(deps.Logger))
	r.Use(RequestID)
	// Phase 5: Tracing
	r.Use(Logging(deps.Logger))
	// Phase 5: Metrics — before rate limiting, so rejected requests are counted.
	r.Use(SecurityHeaders)
	r.Use(CORS(deps.Config.HTTP))
	r.Use(BodyLimit(deps.Config.HTTP.MaxBodyBytes))
	r.Use(Timeout(deps.Config.HTTP.RequestTimeout, deps.Logger))
	if deps.RateLimitByIP != nil {
		r.Use(deps.RateLimitByIP)
	}

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		WriteError(req.Context(), w, deps.Logger,
			apperr.NotFound("ROUTE_NOT_FOUND", "the requested endpoint does not exist"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		// 405 has no counterpart in the error Kind set: it describes the
		// route table, not a business outcome. Written directly, like 504.
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte(
			`{"error":{"code":"METHOD_NOT_ALLOWED","message":"the method is not allowed for this endpoint"}}`))
	})

	// Probes sit outside /api/v1: they are infrastructure, not API surface,
	// and must stay reachable if the API is ever versioned away.
	deps.Health.routes(r)

	r.Route("/api/v1", func(v1 chi.Router) {
		if deps.Register != nil {
			deps.Register(v1)
		}
	})

	return r
}
