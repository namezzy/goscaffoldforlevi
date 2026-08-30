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
	version  string

	// draining is set at the start of shutdown so that /ready fails while the
	// server is still serving in-flight requests. Without this gap the load
	// balancer keeps sending traffic to a socket that is about to close.
	draining atomic.Bool
}

// NewHealth builds the health handler.
func NewHealth(logger *slog.Logger, version string, checkers ...Checker) *Health {
	return &Health{logger: logger, checkers: checkers, version: version}
}

// StartDraining makes /ready report failure. Called first during shutdown.
func (h *Health) StartDraining() { h.draining.Store(true) }

type healthResponse struct {
	Status     string            `json:"status"`
	Version    string            `json:"version"`
	Components map[string]string `json:"components,omitempty"`
}

// Handler mounts the probes on r.
func (h *Health) routes(r chiRouter) {
	r.Get("/health", h.handleHealth)
	r.Get("/live", h.handleLive)
	r.Get("/ready", h.handleReady)
}

func (h *Health) handleHealth(w http.ResponseWriter, r *http.Request) {
	WriteJSON(r.Context(), w, h.logger, http.StatusOK,
		healthResponse{Status: "ok", Version: h.version})
}

func (h *Health) handleLive(w http.ResponseWriter, r *http.Request) {
	// Reaching this line proves the process is scheduling goroutines and
	// serving HTTP. That is the entire meaning of liveness.
	WriteJSON(r.Context(), w, h.logger, http.StatusOK,
		healthResponse{Status: "ok", Version: h.version})
}

func (h *Health) handleReady(w http.ResponseWriter, r *http.Request) {
	if h.draining.Load() {
		WriteJSON(r.Context(), w, h.logger, http.StatusServiceUnavailable,
			healthResponse{Status: "draining", Version: h.version})
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

	WriteJSON(r.Context(), w, h.logger, status,
		healthResponse{Status: label, Version: h.version, Components: components})
}
