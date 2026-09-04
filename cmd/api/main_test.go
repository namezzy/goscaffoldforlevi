//go:build integration

package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/postgres/pgtest"
)

// TestRunServesAndShutsDownGracefully exercises the composition root end to
// end. Unit tests cannot catch the failures that matter here — a dependency
// constructed in the wrong order, a shutdown that hangs, a readiness probe
// that never flips — because all of them are properties of the wiring.
func TestRunServesAndShutsDownGracefully(t *testing.T) {
	c := pgtest.Start(t)
	port := freePort(t)
	// The metrics listener needs a reserved port too. Leaving it at the
	// default would make this test fail whenever a developer happens to have
	// the service running locally, and a test that fails for a reason unrelated
	// to the code under test is a test people learn to ignore.
	metricsPort := freePort(t)

	setEnv(t, map[string]string{
		"APP_ENV":                    "development",
		"HTTP_HOST":                  "127.0.0.1",
		"HTTP_PORT":                  strconv.Itoa(port),
		"HTTP_SHUTDOWN_TIMEOUT":      "10s",
		"HTTP_READINESS_DRAIN_DELAY": "200ms",
		"METRICS_ENABLED":            "true",
		"METRICS_ADDR":               "127.0.0.1:" + strconv.Itoa(metricsPort),
		"DATABASE_URL":               c.URL,
		// Redis is not started: the service must come up anyway, because the
		// cache is optional by configuration.
		"REDIS_REQUIRED": "false",
		"REDIS_ADDR":     "127.0.0.1:1",
		// There is deliberately no default JWT secret: a service that boots
		// with a built-in key is a service with a published key.
		"AUTH_JWT_SECRET": "test-secret-of-at-least-32-bytes-length",
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx) }()

	base := "http://127.0.0.1:" + strconv.Itoa(port)
	waitForReady(t, base, done)

	t.Run("liveness is up", func(t *testing.T) {
		if got := statusOf(t, base+"/live"); got != http.StatusOK {
			t.Fatalf("/live = %d, want 200", got)
		}
	})

	t.Run("unknown routes are structured errors", func(t *testing.T) {
		if got := statusOf(t, base+"/api/v1/nope"); got != http.StatusNotFound {
			t.Fatalf("/api/v1/nope = %d, want 404", got)
		}
	})

	t.Run("metrics are exported on their own port", func(t *testing.T) {
		metricsBase := "http://127.0.0.1:" + strconv.Itoa(metricsPort)

		resp, err := http.Get(metricsBase + "/metrics")
		if err != nil {
			t.Fatalf("GET /metrics: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("/metrics = %d, want 200", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read /metrics: %v", err)
		}
		// The requests above must be visible. An endpoint that serves an empty
		// exposition is indistinguishable from one that serves a live registry
		// until an incident, when the graphs turn out to have always been flat.
		if !strings.Contains(string(body), "http_requests_total") {
			t.Error("/metrics does not expose http_requests_total")
		}

		// The metrics listener carries no authentication, so it must not serve
		// the application. A mux that falls through to the API turns an
		// internal-only port into an unauthenticated copy of the service.
		if got := statusOf(t, metricsBase+"/live"); got != http.StatusNotFound {
			t.Errorf("metrics port served /live with %d, want 404", got)
		}

		// The application port must not export the registry: it is public, and
		// metrics name internal routes, dependencies and error rates.
		if got := statusOf(t, base+"/metrics"); got == http.StatusOK {
			t.Error("the public HTTP port exposes /metrics")
		}
	})

	t.Run("shutdown drains before closing", func(t *testing.T) {
		cancel()

		// While draining, the server must still answer, and /ready must
		// already report failure. That gap is what makes a rolling deploy
		// lose no requests.
		deadline := time.Now().Add(2 * time.Second)
		sawDraining := false
		for time.Now().Before(deadline) {
			code, err := probe(base + "/ready")
			if err != nil {
				break // listener closed; draining window is over
			}
			if code == http.StatusServiceUnavailable {
				sawDraining = true
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !sawDraining {
			t.Error("/ready never reported 503 while draining")
		}

		select {
		case err := <-done:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("run() = %v, want a clean shutdown", err)
			}
		case <-time.After(15 * time.Second):
			t.Fatal("run() did not return within the shutdown budget")
		}
	})
}

// TestRunRejectsInvalidConfiguration verifies the service refuses to start
// rather than starting in a broken state, which is much harder to diagnose.
func TestRunRejectsInvalidConfiguration(t *testing.T) {
	setEnv(t, map[string]string{
		"APP_ENV":         "production",
		"DATABASE_URL":    "postgres://localhost:5432/db",
		"AUTH_JWT_SECRET": "change-me-in-production-0000000000",
	})

	err := run(context.Background())
	if err == nil {
		t.Fatal("run() = nil, want a configuration error")
	}
	if !strings.Contains(err.Error(), "AUTH_JWT_SECRET") {
		t.Fatalf("run() = %v, want it to name the offending key", err)
	}
}

func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

func waitForReady(t *testing.T, base string, done <-chan error) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		// A startup failure must be reported as itself, not as a timeout.
		select {
		case err := <-done:
			t.Fatalf("run() exited during startup: %v", err)
		default:
		}

		if code, err := probe(base + "/ready"); err == nil && code == http.StatusOK {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("server did not become ready")
}

func probe(url string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, nil
}

func statusOf(t *testing.T, url string) int {
	t.Helper()
	code, err := probe(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return code
}
