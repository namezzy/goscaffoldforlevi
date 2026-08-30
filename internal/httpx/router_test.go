package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/go-enterprise/internal/config"
)

func newTestRouter(t *testing.T, checkers ...Checker) http.Handler {
	t.Helper()

	logger := discardLogger()
	return NewRouter(RouterDeps{
		Config: config.Config{HTTP: config.HTTP{
			MaxBodyBytes:   1 << 20,
			RequestTimeout: 5_000_000_000,
		}},
		Logger: logger,
		Health: NewHealth(logger, "test", checkers...),
	})
}

func TestLiveDoesNotDependOnAnything(t *testing.T) {
	// Liveness answers "restart this pod?". Wiring a database check into it
	// turns a database outage into a restart storm across every replica,
	// which is precisely when restarts help least.
	failing := CheckFunc{
		ComponentName: "postgres",
		IsRequired:    true,
		Probe:         func(context.Context) error { return errors.New("down") },
	}

	r := newTestRouter(t, failing)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/live", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("/live = %d while a dependency was down, want 200", rec.Code)
	}
}

func TestReadyFailsWhenARequiredDependencyIsDown(t *testing.T) {
	r := newTestRouter(t, CheckFunc{
		ComponentName: "postgres",
		IsRequired:    true,
		Probe:         func(context.Context) error { return errors.New("connection refused to 10.0.0.5:5432") },
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("/ready = %d, want 503", rec.Code)
	}
	// Probe endpoints are frequently exposed more widely than the API; the
	// failure text must not carry infrastructure addresses.
	if strings.Contains(rec.Body.String(), "10.0.0.5") {
		t.Fatalf("readiness body leaked an address: %s", rec.Body.String())
	}
}

// TestReadyTolerantOfOptionalDependencies pins ADR-0006: a cache outage
// degrades the service, it does not remove it from the load balancer.
func TestReadyTolerantOfOptionalDependencies(t *testing.T) {
	r := newTestRouter(t,
		CheckFunc{
			ComponentName: "postgres",
			IsRequired:    true,
			Probe:         func(context.Context) error { return nil },
		},
		CheckFunc{
			ComponentName: "redis",
			IsRequired:    false,
			Probe:         func(context.Context) error { return errors.New("down") },
		},
	)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("/ready = %d with only an optional dependency down, want 200", rec.Code)
	}

	var parsed struct {
		Components map[string]string `json:"components"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Components["redis"] != "unavailable" {
		t.Fatalf("components = %v, want redis reported as unavailable", parsed.Components)
	}
}

// TestReadyFailsWhileDraining is the check that makes zero-downtime deploys
// real: the pod must advertise itself as unready before it stops listening,
// or the load balancer keeps routing to a closing socket.
func TestReadyFailsWhileDraining(t *testing.T) {
	logger := discardLogger()
	health := NewHealth(logger, "test", CheckFunc{
		ComponentName: "postgres",
		IsRequired:    true,
		Probe:         func(context.Context) error { return nil },
	})

	r := NewRouter(RouterDeps{
		Config: config.Config{HTTP: config.HTTP{MaxBodyBytes: 1 << 20, RequestTimeout: 5_000_000_000}},
		Logger: logger,
		Health: health,
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/ready before draining = %d, want 200", rec.Code)
	}

	health.StartDraining()

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("/ready while draining = %d, want 503", rec.Code)
	}
}

func TestUnknownRouteReturnsAStructuredError(t *testing.T) {
	r := newTestRouter(t)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ROUTE_NOT_FOUND") {
		t.Fatalf("body = %s, want a machine-readable code", rec.Body.String())
	}
}

func TestSecurityHeadersArePresent(t *testing.T) {
	r := newTestRouter(t)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestEveryResponseCarriesARequestID(t *testing.T) {
	r := newTestRouter(t)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Header().Get(HeaderRequestID) == "" {
		t.Fatal("response has no request id header")
	}
}
