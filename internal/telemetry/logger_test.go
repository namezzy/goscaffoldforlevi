package telemetry_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/example/go-enterprise/internal/config"
	"github.com/example/go-enterprise/internal/telemetry"
)

func TestLoggerIncludesBaseFields(t *testing.T) {
	var buf strings.Builder
	logger := telemetry.NewLogger(
		config.App{Name: "svc", Environment: config.EnvProduction, Version: "1.2.3"},
		telemetry.LoggerOptions{Writer: &buf},
	)

	logger.Info("hello")

	var parsed map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &parsed); err != nil {
		t.Fatalf("production logs must be JSON: %v (%s)", err, buf.String())
	}
	for key, want := range map[string]string{
		"service":     "svc",
		"environment": "production",
		"version":     "1.2.3",
	} {
		if parsed[key] != want {
			t.Errorf("%s = %v, want %q", key, parsed[key], want)
		}
	}
}

// TestLoggerAttachesRequestIDFromContext is the reason the handler exists:
// correlation that depends on every call site remembering to pass a field is
// correlation that will be missing exactly when it is needed.
func TestLoggerAttachesRequestIDFromContext(t *testing.T) {
	var buf strings.Builder
	logger := telemetry.NewLogger(
		config.App{Name: "svc", Environment: config.EnvProduction, Version: "1"},
		telemetry.LoggerOptions{Writer: &buf},
	)

	ctx := telemetry.ContextWithRequestID(context.Background(), "abc-123")
	logger.InfoContext(ctx, "hello")

	if !strings.Contains(buf.String(), `"request_id":"abc-123"`) {
		t.Fatalf("request id missing from log line: %s", buf.String())
	}
}

func TestLoggerSurvivesAContextWithoutARequestID(t *testing.T) {
	var buf strings.Builder
	logger := telemetry.NewLogger(
		config.App{Name: "svc", Environment: config.EnvProduction, Version: "1"},
		telemetry.LoggerOptions{Writer: &buf},
	)

	logger.InfoContext(context.Background(), "hello")

	if strings.Contains(buf.String(), "request_id") {
		t.Fatalf("empty request id was emitted: %s", buf.String())
	}
}

func TestRequestIDPropagatesThroughWithAttrs(t *testing.T) {
	// slog.With returns a derived handler; a naive implementation loses the
	// context decoration at that point and the failure is silent.
	var buf strings.Builder
	logger := telemetry.NewLogger(
		config.App{Name: "svc", Environment: config.EnvProduction, Version: "1"},
		telemetry.LoggerOptions{Writer: &buf},
	).With(slog.String("component", "user")).WithGroup("detail")

	ctx := telemetry.ContextWithRequestID(context.Background(), "abc-123")
	logger.InfoContext(ctx, "hello")

	if !strings.Contains(buf.String(), "abc-123") {
		t.Fatalf("request id lost after With/WithGroup: %s", buf.String())
	}
}

func TestDevelopmentUsesTextOutput(t *testing.T) {
	var buf strings.Builder
	logger := telemetry.NewLogger(
		config.App{Name: "svc", Environment: config.EnvDevelopment, Version: "dev"},
		telemetry.LoggerOptions{Writer: &buf},
	)

	logger.Info("hello")

	if strings.HasPrefix(strings.TrimSpace(buf.String()), "{") {
		t.Fatalf("development output is JSON: %s", buf.String())
	}
}

func TestLevelIsHonoured(t *testing.T) {
	var buf strings.Builder
	logger := telemetry.NewLogger(
		config.App{Name: "svc", Environment: config.EnvProduction, Version: "1"},
		telemetry.LoggerOptions{Writer: &buf, Level: slog.LevelWarn},
	)

	logger.Info("suppressed")
	if buf.Len() != 0 {
		t.Fatalf("info was emitted at warn level: %s", buf.String())
	}

	logger.Warn("emitted")
	if buf.Len() == 0 {
		t.Fatal("warn was suppressed at warn level")
	}
}
