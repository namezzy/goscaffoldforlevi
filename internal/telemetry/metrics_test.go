package telemetry_test

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/example/go-enterprise/internal/telemetry"
)

func gather(t *testing.T, m *telemetry.Metrics) []*dto.MetricFamily {
	t.Helper()
	families, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather() = %v", err)
	}
	return families
}

func familyNamed(t *testing.T, m *telemetry.Metrics, name string) *dto.MetricFamily {
	t.Helper()
	for _, f := range gather(t, m) {
		if f.GetName() == name {
			return f
		}
	}
	t.Fatalf("metric %q was never registered", name)
	return nil
}

// The names the dashboards and alerts are written against. Renaming one is not
// a refactor: it silently empties every panel and stops every alert from
// firing, and nothing in the application notices.
func TestTheDocumentedMetricsExist(t *testing.T) {
	m := telemetry.NewMetrics()

	// Give every instrument one observation so it appears in the gather.
	m.HTTPRequest("GET", "/api/v1/users", 200, 0.01)
	m.HTTPRequest("GET", "/api/v1/users", 500, 0.01)
	m.RateLimitRejected("ip")
	m.RedisOperation("get", 0.001, nil)
	m.MessagingPublished("user.created.v1")
	m.MessagingConsumed("user.created.v1", "worker", 0.02, nil)
	m.MessagingRetry("user.created.v1", "worker")
	m.MessagingDeadLettered("user.created.v1", "worker")
	m.UserRegistered()
	m.LoginAttempt(true)
	m.LoginAttempt(false)

	want := []string{
		"http_requests_total",
		"http_request_duration_seconds",
		"http_errors_total",
		"rate_limit_rejected_total",
		"redis_operation_total",
		"redis_operation_duration_seconds",
		"messaging_published_total",
		"messaging_consumed_total",
		"messaging_consume_duration_seconds",
		"messaging_retry_total",
		"messaging_dlq_total",
		"user_registration_total",
		"login_total",
		"login_failure_total",
	}

	have := map[string]bool{}
	for _, f := range gather(t, m) {
		have[f.GetName()] = true
	}
	for _, name := range want {
		if !have[name] {
			t.Errorf("metric %q is missing", name)
		}
	}
}

// §5.11. A label whose value comes from the request is a label an attacker
// chooses. Each distinct value is a permanent time series, so an unbounded
// label is a memory leak with a remote trigger.
func TestNoMetricCarriesAnIdentifierAsALabel(t *testing.T) {
	m := telemetry.NewMetrics()

	m.HTTPRequest("GET", "/api/v1/users/{id}", 200, 0.01)
	m.RedisOperation("get", 0.001, nil)
	m.MessagingConsumed("user.created.v1", "worker", 0.02, nil)
	m.UserRegistered()
	m.LoginAttempt(false)

	// Anything derived from a principal, a request, or a URL.
	banned := []string{
		"user_id", "userid", "user",
		"email", "request_id", "requestid",
		"token", "url", "path", "ip", "client_ip", "trace_id",
	}

	for _, f := range gather(t, m) {
		for _, metric := range f.GetMetric() {
			for _, label := range metric.GetLabel() {
				name := strings.ToLower(label.GetName())
				for _, bad := range banned {
					if name == bad {
						t.Errorf("metric %s has label %q, which is unbounded",
							f.GetName(), label.GetName())
					}
				}
			}
		}
	}
}

// The route label must be the template. A concrete path means one series per
// user id, which is the same unbounded-cardinality failure as labelling by id
// directly — just harder to notice, because it only shows up once the table
// has real rows in it.
func TestTheRouteLabelIsTheTemplateNotTheConcretePath(t *testing.T) {
	m := telemetry.NewMetrics()

	m.HTTPRequest("GET", "/api/v1/users/{id}", 200, 0.01)

	f := familyNamed(t, m, "http_requests_total")
	for _, metric := range f.GetMetric() {
		for _, label := range metric.GetLabel() {
			if label.GetName() != "route" {
				continue
			}
			if !strings.Contains(label.GetValue(), "{id}") {
				t.Errorf("route label = %q, want the template", label.GetValue())
			}
		}
	}
}

// A request that matched no route has no template, and using the raw path
// would let anyone create series by hand: GET /aaaa, GET /aaab, ... until the
// process runs out of memory.
//
// This unit can only enforce half of that: given no template, it must not
// invent one. The other half — that the middleware passes no template rather
// than the path — is not knowable from here, because a route string and a path
// string are the same type. It is tested in internal/httpx, against a real
// router, where the difference is observable.
func TestAnAbsentRouteTemplateCollapsesOntoOneSeries(t *testing.T) {
	m := telemetry.NewMetrics()

	m.HTTPRequest("GET", "", 404, 0.001)
	m.HTTPRequest("GET", "", 404, 0.001)
	m.HTTPRequest("POST", "", 404, 0.001)

	f := familyNamed(t, m, "http_requests_total")
	routes := map[string]bool{}
	for _, metric := range f.GetMetric() {
		for _, label := range metric.GetLabel() {
			if label.GetName() == "route" {
				routes[label.GetValue()] = true
			}
		}
	}

	if len(routes) != 1 {
		t.Fatalf("absent templates produced routes %v, want exactly one", routes)
	}
	if !routes[telemetry.RouteUnmatched] {
		t.Errorf("routes = %v, want %q", routes, telemetry.RouteUnmatched)
	}
}

// Status is a label, so it must be the class and not the code — otherwise a
// handler that invents codes, or a proxy that passes odd ones through, widens
// the label set from outside. Three-digit codes are bounded in theory and
// noisy in practice; the queries people actually write are "5xx rate" anyway.
func TestTheStatusLabelIsAClass(t *testing.T) {
	m := telemetry.NewMetrics()

	m.HTTPRequest("GET", "/api/v1/users", 200, 0.01)
	m.HTTPRequest("GET", "/api/v1/users", 201, 0.01)
	m.HTTPRequest("GET", "/api/v1/users", 204, 0.01)

	f := familyNamed(t, m, "http_requests_total")
	if got := len(f.GetMetric()); got != 1 {
		t.Errorf("200, 201 and 204 produced %d series, want them to share 2xx", got)
	}
}

// http_errors_total exists so alerts do not have to sum a filtered vector.
// Only server faults belong in it: a 404 or a 401 is the API working, and an
// error rate that climbs whenever a client mistypes a URL is an alert people
// learn to ignore.
func TestOnlyServerFaultsCountAsErrors(t *testing.T) {
	m := telemetry.NewMetrics()

	m.HTTPRequest("GET", "/api/v1/users", 404, 0.01)
	m.HTTPRequest("GET", "/api/v1/users", 401, 0.01)
	m.HTTPRequest("GET", "/api/v1/users", 429, 0.01)

	for _, f := range gather(t, m) {
		if f.GetName() != "http_errors_total" {
			continue
		}
		for _, metric := range f.GetMetric() {
			if v := metric.GetCounter().GetValue(); v != 0 {
				t.Errorf("http_errors_total = %v after only 4xx responses, want 0", v)
			}
		}
	}

	m.HTTPRequest("GET", "/api/v1/users", 503, 0.01)
	f := familyNamed(t, m, "http_errors_total")
	total := 0.0
	for _, metric := range f.GetMetric() {
		total += metric.GetCounter().GetValue()
	}
	if total != 1 {
		t.Errorf("http_errors_total = %v after one 503, want 1", total)
	}
}

// A failed login must increment both counters. If login_total only counted
// successes, "failure rate" would be failures over successes, which reads
// below 100%% during an outage where every attempt fails.
func TestAFailedLoginCountsAsAnAttemptAndAFailure(t *testing.T) {
	m := telemetry.NewMetrics()

	m.LoginAttempt(false)

	attempts := counterValue(t, m, "login_total")
	failures := counterValue(t, m, "login_failure_total")

	if attempts != 1 {
		t.Errorf("login_total = %v, want 1", attempts)
	}
	if failures != 1 {
		t.Errorf("login_failure_total = %v, want 1", failures)
	}
}

func TestASuccessfulLoginIsNotAFailure(t *testing.T) {
	m := telemetry.NewMetrics()

	m.LoginAttempt(true)

	if got := counterValue(t, m, "login_total"); got != 1 {
		t.Errorf("login_total = %v, want 1", got)
	}
	if got := counterValue(t, m, "login_failure_total"); got != 0 {
		t.Errorf("login_failure_total = %v, want 0", got)
	}
}

// Two processes in one test binary, or an application and its tests, must not
// fight over a package-level registry. Prometheus panics on duplicate
// registration, so a global would turn a second construction into a crash.
func TestTwoRegistriesDoNotCollide(t *testing.T) {
	a := telemetry.NewMetrics()
	b := telemetry.NewMetrics()

	a.UserRegistered()

	if got := counterValue(t, a, "user_registration_total"); got != 1 {
		t.Errorf("first registry = %v, want 1", got)
	}
	if got := counterValue(t, b, "user_registration_total"); got != 0 {
		t.Errorf("second registry = %v, want 0; registries are sharing state", got)
	}
}

// The registry must not include the default collectors' process and Go runtime
// metrics by accident, but it must include them on purpose: db pool saturation
// and goroutine leaks are the two things a Go service dies of, and neither is
// visible from request metrics.
func TestTheRegistryIncludesRuntimeAndProcessMetrics(t *testing.T) {
	m := telemetry.NewMetrics()

	have := map[string]bool{}
	for _, f := range gather(t, m) {
		have[f.GetName()] = true
	}

	for _, name := range []string{"go_goroutines", "go_memstats_alloc_bytes"} {
		if !have[name] {
			t.Errorf("%q is missing; goroutine leaks would be invisible", name)
		}
	}
}

// db_pool_* comes from pgxpool's own counters, which only exist once there is
// a pool. Registering the collector must therefore be a separate step, and it
// must be safe to skip when the process has no database — cmd/adminctl and the
// tests both run without one.
func TestDatabasePoolMetricsAppearOnlyOnceAPoolIsRegistered(t *testing.T) {
	m := telemetry.NewMetrics()

	if hasFamily(t, m, "db_pool_total_conns") {
		t.Error("db_pool_total_conns exists before any pool was registered")
	}

	m.RegisterPoolCollector(stubPoolStats{total: 7, idle: 3, acquired: 4})

	if !hasFamily(t, m, "db_pool_total_conns") {
		t.Fatal("db_pool_total_conns is missing after registering a pool")
	}
	if got := gaugeValue(t, m, "db_pool_total_conns"); got != 7 {
		t.Errorf("db_pool_total_conns = %v, want 7", got)
	}
}

type stubPoolStats struct{ total, idle, acquired int32 }

func (s stubPoolStats) TotalConns() int32    { return s.total }
func (s stubPoolStats) IdleConns() int32     { return s.idle }
func (s stubPoolStats) AcquiredConns() int32 { return s.acquired }

func hasFamily(t *testing.T, m *telemetry.Metrics, name string) bool {
	t.Helper()
	for _, f := range gather(t, m) {
		if f.GetName() == name {
			return true
		}
	}
	return false
}

func counterValue(t *testing.T, m *telemetry.Metrics, name string) float64 {
	t.Helper()
	total := 0.0
	for _, f := range gather(t, m) {
		if f.GetName() != name {
			continue
		}
		for _, metric := range f.GetMetric() {
			total += metric.GetCounter().GetValue()
		}
	}
	return total
}

func gaugeValue(t *testing.T, m *telemetry.Metrics, name string) float64 {
	t.Helper()
	f := familyNamed(t, m, name)
	total := 0.0
	for _, metric := range f.GetMetric() {
		total += metric.GetGauge().GetValue()
	}
	return total
}

var _ prometheus.Gatherer = (*prometheus.Registry)(nil)
