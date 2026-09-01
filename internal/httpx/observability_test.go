package httpx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/example/go-enterprise/internal/config"
	"github.com/example/go-enterprise/internal/httpx"
	"github.com/example/go-enterprise/internal/messaging"
	"github.com/example/go-enterprise/internal/telemetry"
)

// recordingTracer gives the tests a real SDK provider writing to memory, so
// assertions are made against spans as an exporter would see them rather than
// against the instrumentation's intent.
func recordingTracer(t *testing.T) (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return tp, rec
}

// routerWith mounts a tiny route tree carrying the middleware under test, so
// that chi's route pattern is populated the way it is in production. A bare
// handler would let a broken template assertion pass, because there would be
// no template to get wrong.
func routerWith(mw func(http.Handler) http.Handler, h http.HandlerFunc) http.Handler {
	r := chi.NewRouter()
	r.Use(mw)
	r.Get("/api/v1/users/{id}", h)
	r.Get("/api/v1/users", h)
	return r
}

func spanAttr(span sdktrace.ReadOnlySpan, key attribute.Key) string {
	for _, kv := range span.Attributes() {
		if kv.Key == key {
			return kv.Value.String()
		}
	}
	return ""
}

// §5.3. The span name must be the route template. Naming it after the concrete
// path gives the trace backend one operation per user id, which makes "how
// slow is GET /users/{id}" unanswerable and eventually exhausts the backend's
// own index.
func TestTheSpanIsNamedAfterTheRouteTemplate(t *testing.T) {
	tp, rec := recordingTracer(t)

	h := routerWith(httpx.Tracing(tp), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/users/8c91e0f2-2f7d-4c39-9c9f-6f4d1f1f0b1a", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}

	if got, want := spans[0].Name(), "GET /api/v1/users/{id}"; got != want {
		t.Errorf("span name = %q, want %q", got, want)
	}
	if strings.Contains(spans[0].Name(), "8c91e0f2") {
		t.Errorf("span name contains the concrete id: %q", spans[0].Name())
	}
}

func TestTheSpanRecordsMethodRouteAndStatus(t *testing.T) {
	tp, rec := recordingTracer(t)

	h := routerWith(httpx.Tracing(tp), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	h.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/api/v1/users", nil))

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}

	for key, want := range map[attribute.Key]string{
		"http.request.method":       "GET",
		"http.route":                "/api/v1/users",
		"http.response.status_code": "201",
	} {
		if got := spanAttr(spans[0], key); got != want {
			t.Errorf("attribute %s = %q, want %q", key, got, want)
		}
	}
}

// A 500 must set the span status to Error, because that is what the trace
// backend filters on. A 404 must not: it is the API working, and a trace list
// where every mistyped URL is red teaches people to ignore red.
func TestOnlyServerFaultsMarkTheSpanAsFailed(t *testing.T) {
	cases := []struct {
		status int
		want   codes.Code
	}{
		{http.StatusOK, codes.Unset},
		{http.StatusNotFound, codes.Unset},
		{http.StatusTooManyRequests, codes.Unset},
		{http.StatusInternalServerError, codes.Error},
		{http.StatusServiceUnavailable, codes.Error},
	}

	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			tp, rec := recordingTracer(t)

			h := routerWith(httpx.Tracing(tp), func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			})
			h.ServeHTTP(httptest.NewRecorder(),
				httptest.NewRequest(http.MethodGet, "/api/v1/users", nil))

			spans := rec.Ended()
			if len(spans) != 1 {
				t.Fatalf("recorded %d spans, want 1", len(spans))
			}
			if got := spans[0].Status().Code; got != tc.want {
				t.Errorf("span status = %v, want %v", got, tc.want)
			}
		})
	}
}

// §5.4. An inbound traceparent must continue the caller's trace rather than
// start a new one. Getting this wrong produces two disconnected traces that
// each look complete, which is harder to notice than no tracing at all.
func TestAnInboundTraceparentContinuesTheSameTrace(t *testing.T) {
	tp, rec := recordingTracer(t)

	const (
		traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
		spanID  = "00f067aa0ba902b7"
	)

	h := routerWith(httpx.Tracing(tp), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("traceparent", "00-"+traceID+"-"+spanID+"-01")
	h.ServeHTTP(httptest.NewRecorder(), req)

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	if got := spans[0].SpanContext().TraceID().String(); got != traceID {
		t.Errorf("trace id = %q, want the caller's %q", got, traceID)
	}
	if got := spans[0].Parent().SpanID().String(); got != spanID {
		t.Errorf("parent span id = %q, want the caller's %q", got, spanID)
	}
}

// §5.7. Events published while handling a request must carry the *server
// span* as their parent, not the raw inbound header. Forwarding the header
// would link the consumer to the caller and skip this service entirely, so the
// trace would show a publish that no request appears to have caused.
func TestPublishedEventsInheritTheServerSpanNotTheInboundHeader(t *testing.T) {
	tp, rec := recordingTracer(t)

	var seen string
	h := routerWith(httpx.Tracing(tp), func(w http.ResponseWriter, r *http.Request) {
		seen = messaging.TraceParentFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	const inbound = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("traceparent", inbound)
	h.ServeHTTP(httptest.NewRecorder(), req)

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	sc := spans[0].SpanContext()

	if seen == inbound {
		t.Fatal("the handler saw the inbound header verbatim; " +
			"a consumer parented on it would not be linked to this request")
	}
	want := "00-" + sc.TraceID().String() + "-" + sc.SpanID().String() + "-01"
	if seen != want {
		t.Errorf("traceparent in context = %q, want the server span %q", seen, want)
	}
}

// With tracing disabled there is no span to derive a traceparent from, but the
// caller's header is still the best available correlation. Dropping it would
// break the outbox's trace column for every deployment that runs without a
// collector, which is the default one.
func TestTheInboundTraceparentIsStillForwardedWhenTracingIsDisabled(t *testing.T) {
	noop, err := telemetry.NewTracerProvider(context.Background(),
		testAppConfig(), noopOTel())
	if err != nil {
		t.Fatalf("NewTracerProvider() = %v", err)
	}

	var seen string
	h := routerWith(httpx.Tracing(noop), func(w http.ResponseWriter, r *http.Request) {
		seen = messaging.TraceParentFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	const inbound = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("traceparent", inbound)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if seen != inbound {
		t.Errorf("traceparent in context = %q, want the forwarded %q", seen, inbound)
	}
}

// §5.1. Disabled observability must not change behaviour. A middleware that
// only works when a collector is configured is a middleware that is tested in
// one mode and deployed in the other.
func TestTracingDisabledLeavesTheResponseUnchanged(t *testing.T) {
	noop, err := telemetry.NewTracerProvider(context.Background(),
		testAppConfig(), noopOTel())
	if err != nil {
		t.Fatalf("NewTracerProvider() = %v", err)
	}

	h := routerWith(httpx.Tracing(noop), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/users", nil))

	if rr.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusTeapot)
	}
	if rr.Body.String() != `{"ok":true}` {
		t.Errorf("body = %q, want it untouched", rr.Body.String())
	}
}

// --- metrics middleware ---

// The counterpart to the telemetry unit test: that one proves an absent
// template collapses, this one proves the middleware passes a template rather
// than the path. Only a real router can tell the two apart.
func TestTheMetricsMiddlewareLabelsByRouteTemplate(t *testing.T) {
	m := telemetry.NewMetrics()

	h := routerWith(httpx.Metrics(m), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, id := range []string{"aaaa", "bbbb", "cccc"} {
		h.ServeHTTP(httptest.NewRecorder(),
			httptest.NewRequest(http.MethodGet, "/api/v1/users/"+id, nil))
	}

	routes := routeLabels(t, m, "http_requests_total")
	if len(routes) != 1 {
		t.Fatalf("three ids produced routes %v, want one template", routes)
	}
	if !routes["/api/v1/users/{id}"] {
		t.Errorf("routes = %v, want /api/v1/users/{id}", routes)
	}
}

// The cardinality bomb §5.11 exists to stop: unrouted requests must not each
// become a series. An attacker controls this path completely, so the series
// count must not depend on it.
func TestUnroutedRequestsDoNotCreateSeries(t *testing.T) {
	m := telemetry.NewMetrics()

	h := routerWith(httpx.Metrics(m), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, path := range []string{"/aaaa", "/bbbb", "/cccc", "/dddd/eeee"} {
		h.ServeHTTP(httptest.NewRecorder(),
			httptest.NewRequest(http.MethodGet, path, nil))
	}

	routes := routeLabels(t, m, "http_requests_total")
	for route := range routes {
		if strings.Contains(route, "aaaa") || strings.Contains(route, "dddd") {
			t.Errorf("route label %q comes from the request path", route)
		}
	}
	if len(routes) != 1 || !routes[telemetry.RouteUnmatched] {
		t.Errorf("routes = %v, want only %q", routes, telemetry.RouteUnmatched)
	}
}

func TestTheMetricsMiddlewareLeavesTheResponseUnchanged(t *testing.T) {
	m := telemetry.NewMetrics()

	h := routerWith(httpx.Metrics(m), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/users", nil))

	if rr.Code != http.StatusTeapot || rr.Body.String() != `{"ok":true}` {
		t.Errorf("response = %d %q, want it untouched", rr.Code, rr.Body.String())
	}
}

// --- helpers ---

func testAppConfig() config.App {
	return config.App{
		Name:        "go-enterprise",
		Environment: config.EnvProduction,
		Version:     "test",
	}
}

func noopOTel() config.OTel {
	return config.OTel{Enabled: false, Protocol: config.OTLPGRPC}
}

func routeLabels(t *testing.T, m *telemetry.Metrics, family string) map[string]bool {
	t.Helper()

	families, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather() = %v", err)
	}

	routes := map[string]bool{}
	for _, f := range families {
		if f.GetName() != family {
			continue
		}
		for _, metric := range f.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "route" {
					routes[label.GetValue()] = true
				}
			}
		}
	}
	return routes
}
