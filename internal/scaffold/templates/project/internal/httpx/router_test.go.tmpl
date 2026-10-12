package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/namezzy/goscaffoldforlevi/internal/config"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/telemetry"
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
		Health: NewHealth(logger, Build{Version: "test", Commit: "testcommit"}, checkers...),
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
	health := NewHealth(logger, Build{Version: "test", Commit: "testcommit"}, CheckFunc{
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

// TestMethodNotAllowedAdvertisesTheAllowedMethods covers a defect found by
// sending DELETE to a route that only serves GET and POST: the answer was 405
// with no Allow header.
//
// RFC 9110 requires a 405 to carry Allow, and the reason is practical rather
// than ceremonial: without it the client is told "not that method" and has to
// guess which one to use. chi sets the header in its built-in responder, so
// replacing that responder to emit a JSON body silently dropped it.
func TestMethodNotAllowedAdvertisesTheAllowedMethods(t *testing.T) {
	t.Parallel()

	logger := discardLogger()
	router := NewRouter(RouterDeps{
		Config: config.Config{HTTP: config.HTTP{MaxBodyBytes: 1 << 20, RequestTimeout: 5_000_000_000}},
		Logger: logger,
		Health: NewHealth(logger, Build{Version: "test", Commit: "testcommit"}),
		Register: func(v1 chi.Router) {
			v1.Route("/things", func(r chi.Router) {
				r.Get("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
				r.Post("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })
				r.Get("/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
				r.Delete("/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
			})
		},
	})

	for _, tc := range []struct {
		method, path string
		want         []string
	}{
		{http.MethodDelete, "/api/v1/things", []string{"GET", "POST"}},
		{http.MethodPut, "/api/v1/things", []string{"GET", "POST"}},
		{http.MethodPost, "/api/v1/things/7", []string{"DELETE", "GET"}},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s %s: status = %d, want 405", tc.method, tc.path, rec.Code)
		}

		got := rec.Header().Values("Allow")
		if len(got) == 1 {
			got = strings.Split(got[0], ", ")
		}
		sort.Strings(got)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s %s: Allow = %v, want %v", tc.method, tc.path, got, tc.want)
		}
		// The body still has to be the JSON shape every other error uses; the
		// header is an addition, not a replacement.
		if !strings.Contains(rec.Body.String(), "METHOD_NOT_ALLOWED") {
			t.Errorf("%s %s: body = %s", tc.method, tc.path, rec.Body)
		}
	}

	// A path that matches nothing is a 404, and a 404 must not claim that some
	// other method would have worked.
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/nothing-here", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != "" {
		t.Errorf("a 404 carried Allow = %q", got)
	}
}

// TestFlushReachesTheRealResponseWriter is the regression for a streaming
// handler (an SSE feed, in practice) that silently could not flush: every
// request passes through jsonMethodNotAllowed's own wrapper, and that
// wrapper embedded http.ResponseWriter without an Unwrap method, which is
// what http.ResponseController needs to see past it to the Flusher several
// layers further down. The handler never learned why — it just always got
// http.ErrNotSupported, on every route, regardless of which one actually
// hit the 405 path this wrapper exists for.
func TestFlushReachesTheRealResponseWriter(t *testing.T) {
	t.Parallel()

	logger := discardLogger()
	var flushErr error
	router := NewRouter(RouterDeps{
		Config: config.Config{HTTP: config.HTTP{MaxBodyBytes: 1 << 20, RequestTimeout: 5_000_000_000}},
		Logger: logger,
		Health: NewHealth(logger, Build{Version: "test", Commit: "testcommit"}),
		Register: func(v1 chi.Router) {
			v1.Get("/stream", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				flushErr = http.NewResponseController(w).Flush()
			})
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if flushErr != nil {
		t.Errorf("Flush() = %v, want a handler mounted through the router to reach a real Flusher", flushErr)
	}
}

// TestTheRouterPutsTheRequestTraceOnTheContext checks that TraceContext is
// actually mounted, not merely written. A middleware that exists, is correct
// and is never added to the stack behaves exactly like one that was never
// written: every event would ship with an empty traceparent and no test of the
// middleware in isolation would notice.
func TestTheRouterPutsTheRequestTraceOnTheContext(t *testing.T) {
	t.Parallel()

	const header = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

	logger := discardLogger()
	var seen string
	router := NewRouter(RouterDeps{
		Config: config.Config{HTTP: config.HTTP{MaxBodyBytes: 1 << 20, RequestTimeout: 5_000_000_000}},
		Logger: logger,
		Health: NewHealth(logger, Build{Version: "test", Commit: "testcommit"}),
		Register: func(v1 chi.Router) {
			v1.Get("/traced", func(w http.ResponseWriter, r *http.Request) {
				seen = messaging.TraceParentFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/traced", nil)
	req.Header.Set("traceparent", header)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if seen != header {
		t.Errorf("traceparent reaching the handler = %q, want %q; "+
			"the TraceContext middleware is not mounted on the router", seen, header)
	}
}

// The Phase 4 lesson, applied to Phase 5: a middleware that is written,
// correct and never mounted behaves exactly like one that was never written.
// Both of these assert against the router rather than the middleware, because
// mounting is the part no unit test can see.
func TestTheRouterRecordsMetricsForEveryRequest(t *testing.T) {
	t.Parallel()

	logger := discardLogger()
	metrics := telemetry.NewMetrics()
	router := NewRouter(RouterDeps{
		Config:  config.Config{HTTP: config.HTTP{MaxBodyBytes: 1 << 20, RequestTimeout: 5_000_000_000}},
		Logger:  logger,
		Health:  NewHealth(logger, Build{Version: "test", Commit: "testcommit"}),
		Metrics: metrics,
		Register: func(v1 chi.Router) {
			v1.Get("/counted", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
		},
	})

	router.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/api/v1/counted", nil))

	if got := totalOf(t, metrics, "http_requests_total"); got != 1 {
		t.Errorf("http_requests_total = %v, want 1; "+
			"the Metrics middleware is not mounted on the router", got)
	}
}

// §5.9 and the ordering note in NewRouter: the metrics middleware sits before
// the rate limiters so that rejected requests are counted. Moving it after
// them would make a service under attack look idle, which is the one time the
// dashboard has to be right.
func TestRateLimitedRequestsAreStillCounted(t *testing.T) {
	t.Parallel()

	logger := discardLogger()
	metrics := telemetry.NewMetrics()
	router := NewRouter(RouterDeps{
		Config:  config.Config{HTTP: config.HTTP{MaxBodyBytes: 1 << 20, RequestTimeout: 5_000_000_000}},
		Logger:  logger,
		Health:  NewHealth(logger, Build{Version: "test", Commit: "testcommit"}),
		Metrics: metrics,
		RateLimitByIP: func(http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTooManyRequests)
			})
		},
		Register: func(v1 chi.Router) {
			v1.Get("/counted", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
		},
	})

	router.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/api/v1/counted", nil))

	if got := totalOf(t, metrics, "http_requests_total"); got != 1 {
		t.Errorf("http_requests_total = %v after a 429, want 1; "+
			"rejected traffic is invisible", got)
	}
}

// A router built without a metrics registry or a tracer provider must still
// serve. Every existing test constructs RouterDeps without them, and more to
// the point cmd/adminctl and any future entry point should not have to
// assemble an observability stack to mount a route.
func TestTheRouterWorksWithoutObservabilityWiredIn(t *testing.T) {
	t.Parallel()

	logger := discardLogger()
	router := NewRouter(RouterDeps{
		Config: config.Config{HTTP: config.HTTP{MaxBodyBytes: 1 << 20, RequestTimeout: 5_000_000_000}},
		Logger: logger,
		Health: NewHealth(logger, Build{Version: "test", Commit: "testcommit"}),
		Register: func(v1 chi.Router) {
			v1.Get("/plain", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
		},
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/plain", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 with no telemetry configured", rec.Code)
	}
}

func totalOf(t *testing.T, m *telemetry.Metrics, name string) float64 {
	t.Helper()

	families, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather() = %v", err)
	}

	total := 0.0
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, metric := range f.GetMetric() {
			total += metric.GetCounter().GetValue()
		}
	}
	return total
}

// TestProbesAreNotRateLimited pins the exemption that keeps the rate limiter
// from taking the service down by itself.
//
// The failure it prevents is indirect and therefore hard to diagnose: the
// kubelet probes from the node's address, every pod on that node shares one
// quota bucket, and a 429 on /ready is a probe failure — so the pods are marked
// NotReady and removed from the Service. Nothing logs "rate limited the
// kubelet"; the cluster simply flaps.
func TestProbesAreNotRateLimited(t *testing.T) {
	logger := discardLogger()

	// A limiter that refuses everything. If a probe reaches it, the probe fails.
	refuseEverything := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		})
	}

	router := NewRouter(RouterDeps{
		Config:        config.Config{HTTP: config.HTTP{MaxBodyBytes: 1 << 20, RequestTimeout: 5_000_000_000}},
		Logger:        logger,
		Health:        NewHealth(logger, Build{Version: "test", Commit: "testcommit"}),
		RateLimitByIP: refuseEverything,
	})

	for _, path := range []string{"/health", "/live", "/ready"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			if rec.Code == http.StatusTooManyRequests {
				t.Fatalf("%s was rate limited; a 429 here is a probe failure and the pod is withdrawn", path)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("%s answered %d, want 200", path, rec.Code)
			}
		})
	}

	// The exemption must be exactly that narrow: everything else still pays.
	// Otherwise a flood of unknown paths costs nothing.
	for _, path := range []string{"/api/v1/users", "/nope"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			if rec.Code != http.StatusTooManyRequests {
				t.Errorf("%s answered %d; it should still be rate limited", path, rec.Code)
			}
		})
	}
}
