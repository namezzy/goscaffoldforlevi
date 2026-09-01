package telemetry_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/example/go-enterprise/internal/config"
	"github.com/example/go-enterprise/internal/telemetry"
)

func startEndpoint(t *testing.T, cfg config.Metrics, m *telemetry.Metrics) string {
	t.Helper()

	srv, addr, err := telemetry.NewMetricsServer(context.Background(), cfg, m)
	if err != nil {
		t.Fatalf("NewMetricsServer() = %v", err)
	}
	if srv == nil {
		return ""
	}
	go func() { _ = srv.Serve() }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})
	return addr
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("NewRequest(%s) = %v", url, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s = %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// §5.10. Metrics are served from their own listener, not from the public
// router. On the public port the endpoint is one misconfigured ingress rule
// away from the internet, and it publishes the shape of the system: which
// routes exist, how many users have registered, how many logins are failing.
func TestMetricsAreServedOnTheirOwnPort(t *testing.T) {
	m := telemetry.NewMetrics()
	m.UserRegistered()

	addr := startEndpoint(t, config.Metrics{
		Enabled: true, Addr: "127.0.0.1:0", Path: "/metrics",
	}, m)

	status, body := get(t, "http://"+addr+"/metrics")
	if status != http.StatusOK {
		t.Fatalf("GET /metrics = %d, want 200", status)
	}
	if !strings.Contains(body, "user_registration_total 1") {
		t.Errorf("the exposition does not carry the counter:\n%s", body)
	}
}

// Nothing but the metrics path may be served. A metrics listener that answers
// other paths is an unauthenticated HTTP surface inside the network with no
// reason to exist, and it invites someone to mount a debug handler on it.
func TestTheMetricsListenerServesNothingElse(t *testing.T) {
	addr := startEndpoint(t, config.Metrics{
		Enabled: true, Addr: "127.0.0.1:0", Path: "/metrics",
	}, telemetry.NewMetrics())

	for _, path := range []string{"/", "/healthz", "/debug/pprof/", "/v1/users"} {
		status, _ := get(t, "http://"+addr+path)
		if status != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, status)
		}
	}
}

// Disabling metrics must produce no listener at all. Returning a server that
// is merely never started would still be something a caller can start, and
// "off" has to mean no open port — that is the difference between a flag and a
// promise.
func TestDisablingMetricsOpensNoPort(t *testing.T) {
	srv, addr, err := telemetry.NewMetricsServer(context.Background(), config.Metrics{
		Enabled: false, Addr: "127.0.0.1:0", Path: "/metrics",
	}, telemetry.NewMetrics())
	if err != nil {
		t.Fatalf("NewMetricsServer() = %v", err)
	}
	if srv != nil {
		t.Errorf("a disabled metrics endpoint returned a server")
	}
	if addr != "" {
		t.Errorf("a disabled metrics endpoint returned address %q", addr)
	}
}

// The listener must be opened by the constructor, not by Serve. A port taken
// by another process has to fail startup, where an orchestrator will notice;
// discovered inside a goroutine it becomes a log line in a process that
// reports itself healthy and exports nothing.
func TestABusyPortFailsStartupRatherThanAGoroutine(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	defer taken.Close()

	_, _, err = telemetry.NewMetricsServer(context.Background(), config.Metrics{
		Enabled: true, Addr: taken.Addr().String(), Path: "/metrics",
	}, telemetry.NewMetrics())
	if err == nil {
		t.Error("NewMetricsServer() = nil error for a port already in use")
	}
}

// The configured path must be honoured, or an operator who moves the endpoint
// gets a scrape target that silently returns 404 and a dashboard of gaps.
func TestTheConfiguredPathIsHonoured(t *testing.T) {
	addr := startEndpoint(t, config.Metrics{
		Enabled: true, Addr: "127.0.0.1:0", Path: "/internal/telemetry",
	}, telemetry.NewMetrics())

	if status, _ := get(t, "http://"+addr+"/internal/telemetry"); status != http.StatusOK {
		t.Errorf("GET the configured path = %d, want 200", status)
	}
	if status, _ := get(t, "http://"+addr+"/metrics"); status != http.StatusNotFound {
		t.Errorf("GET /metrics = %d, want 404 once the path was moved", status)
	}
}
