package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// checkTimeout bounds each dependency probe so that a hung dependency cannot
// make the readiness endpoint itself hang, which would look identical to the
// pod being dead.
const checkTimeout = 2 * time.Second

// Checker reports whether a dependency is usable.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
	// Required reports whether a failure should make the pod unready. An
	// optional dependency is reported in the body but does not fail the probe.
	Required() bool
}

// CheckFunc adapts a function to Checker.
type CheckFunc struct {
	ComponentName string
	IsRequired    bool
	Probe         func(ctx context.Context) error
}

// Name returns the component name reported in the readiness body.
func (c CheckFunc) Name() string { return c.ComponentName }

// Required reports whether a failure should make the pod unready.
func (c CheckFunc) Required() bool { return c.IsRequired }

// Check probes the component.
func (c CheckFunc) Check(ctx context.Context) error { return c.Probe(ctx) }

// Health serves the three probe endpoints.
//
// They answer three different questions and must not be collapsed into one:
//
//   - /health  is for humans and uptime monitors: is the process up?
//   - /live    is for the kubelet: should this pod be restarted? It must never
//     depend on the database, or a database outage would trigger a
//     restart storm that makes recovery harder.
//   - /ready   is for the load balancer: should traffic be routed here? It
//     checks required dependencies and reports optional ones.
type Health struct {
	logger   *slog.Logger
	checkers []Checker
	build    Build

	// draining is set at the start of shutdown so that /ready fails while the
	// server is still serving in-flight requests. Without this gap the load
	// balancer keeps sending traffic to a socket that is about to close.
	draining atomic.Bool
}

// Build identifies the running binary. It is passed in rather than read from
// internal/buildinfo directly so that a test can state the values it expects
// instead of depending on how the test binary happened to be linked.
type Build struct {
	Version string
	Commit  string
}

// NewHealth builds the health handler.
func NewHealth(logger *slog.Logger, build Build, checkers ...Checker) *Health {
	return &Health{logger: logger, checkers: checkers, build: build}
}

// StartDraining makes /ready report failure. Called first during shutdown.
func (h *Health) StartDraining() { h.draining.Store(true) }

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`

	// Commit is what makes the report actionable. Two deployments can report
	// the same version while running different code — during development the
	// version does not move at all — so the SHA is the only field that answers
	// "which build is this?" without ambiguity. It names a public commit and
	// discloses nothing an attacker cannot read in the repository.
	Commit string `json:"commit"`

	Components map[string]string `json:"components,omitempty"`
}

// report fills in the fields every probe response shares.
func (h *Health) report(status string) healthResponse {
	return healthResponse{Status: status, Version: h.build.Version, Commit: h.build.Commit}
}

// handlers is the probe table.
//
// It exists so that the set of probe paths has one definition. The router both
// mounts these and exempts them from rate limiting, and a second hand-written
// list would eventually disagree with this one — silently, because a probe that
// is mounted but not exempt still answers, right up until it is refused.
func (h *Health) handlers() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"/health": h.handleHealth,
		"/live":   h.handleLive,
		"/ready":  h.handleReady,
	}
}

// paths returns the probe endpoints, for callers that need to treat them
// differently from API surface.
func (h *Health) paths() map[string]bool {
	paths := make(map[string]bool, len(h.handlers()))
	for path := range h.handlers() {
		paths[path] = true
	}
	return paths
}

// routes mounts the probes on r.
func (h *Health) routes(r chiRouter) {
	for path, handler := range h.handlers() {
		r.Get(path, handler)
	}
}

func (h *Health) handleHealth(w http.ResponseWriter, r *http.Request) {
	WriteJSON(r.Context(), w, h.logger, http.StatusOK, h.report("ok"))
}

func (h *Health) handleLive(w http.ResponseWriter, r *http.Request) {
	// Reaching this line proves the process is scheduling goroutines and
	// serving HTTP. That is the entire meaning of liveness.
	WriteJSON(r.Context(), w, h.logger, http.StatusOK, h.report("ok"))
}

func (h *Health) handleReady(w http.ResponseWriter, r *http.Request) {
	if h.draining.Load() {
		WriteJSON(r.Context(), w, h.logger, http.StatusServiceUnavailable, h.report("draining"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), checkTimeout)
	defer cancel()

	components := make(map[string]string, len(h.checkers))
	ready := true

	for _, c := range h.checkers {
		if err := c.Check(ctx); err != nil {
			// The error text can contain a DSN, so only the component name and
			// a generic marker go into the response; details go to the log.
			h.logger.WarnContext(ctx, "readiness check failed",
				slog.String("component", c.Name()),
				slog.Bool("required", c.Required()),
				slog.Any("error", err),
			)
			components[c.Name()] = "unavailable"
			if c.Required() {
				ready = false
			}
			continue
		}
		components[c.Name()] = "ok"
	}

	status := http.StatusOK
	label := "ok"
	if !ready {
		status = http.StatusServiceUnavailable
		label = "unavailable"
	}

	resp := h.report(label)
	resp.Components = components
	WriteJSON(r.Context(), w, h.logger, status, resp)
}
