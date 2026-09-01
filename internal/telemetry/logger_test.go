package telemetry_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

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

// --- Phase 5: trace correlation ---

// §5.8. The point of putting this in the handler is that no service ever calls
// trace.SpanFromContext. A rule that says "remember to log the trace id" is a
// rule that is followed in the code people were thinking about and skipped in
// the error path, which is the only path where the correlation is needed.
func TestLoggerAttachesTraceAndSpanIDsFromContext(t *testing.T) {
	var buf strings.Builder
	logger := telemetry.NewLogger(
		config.App{Name: "svc", Environment: config.EnvProduction, Version: "1"},
		telemetry.LoggerOptions{Writer: &buf},
	)

	tp, err := telemetry.NewTracerProvider(context.Background(),
		config.App{Name: "svc", Environment: config.EnvProduction, Version: "1"},
		config.OTel{Enabled: true, Endpoint: "203.0.113.1:4317",
			Protocol: config.OTLPGRPC, Insecure: true, SamplingRatio: 1})
	if err != nil {
		t.Fatalf("NewTracerProvider() = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx)
	})

	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	logger.InfoContext(ctx, "hello")

	var parsed map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &parsed); err != nil {
		t.Fatalf("log line is not JSON: %v (%s)", err, buf.String())
	}

	sc := span.SpanContext()
	if parsed["trace_id"] != sc.TraceID().String() {
		t.Errorf("trace_id = %v, want %q", parsed["trace_id"], sc.TraceID())
	}
	if parsed["span_id"] != sc.SpanID().String() {
		t.Errorf("span_id = %v, want %q", parsed["span_id"], sc.SpanID())
	}
}

// With tracing off, spans are non-recording and their ids are all zeroes.
// Logging those would be worse than logging nothing: every line in the system
// would share one trace id, and anyone who pasted it into the trace backend
// would get an empty result rather than an obvious "there is no trace here".
func TestLoggerOmitsTraceIDsWhenTheSpanIsNotSampled(t *testing.T) {
	var buf strings.Builder
	logger := telemetry.NewLogger(
		config.App{Name: "svc", Environment: config.EnvProduction, Version: "1"},
		telemetry.LoggerOptions{Writer: &buf},
	)

	tp, err := telemetry.NewTracerProvider(context.Background(),
		config.App{Name: "svc", Environment: config.EnvProduction, Version: "1"},
		config.OTel{Enabled: false})
	if err != nil {
		t.Fatalf("NewTracerProvider() = %v", err)
	}
	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	logger.InfoContext(ctx, "hello")

	line := buf.String()
	if strings.Contains(line, "trace_id") {
		t.Errorf("log line carries a trace_id with tracing disabled: %s", line)
	}
	if strings.Contains(line, "00000000000000000000000000000000") {
		t.Errorf("log line carries an all-zero trace id: %s", line)
	}
}

// Both identifiers must survive together. request_id is what a support ticket
// carries and trace_id is what the trace backend indexes; losing either one
// breaks the path from "a customer complained" to "here is the span".
func TestLoggerKeepsRequestIDAndTraceIDTogether(t *testing.T) {
	var buf strings.Builder
	logger := telemetry.NewLogger(
		config.App{Name: "svc", Environment: config.EnvProduction, Version: "1"},
		telemetry.LoggerOptions{Writer: &buf},
	)

	tp, err := telemetry.NewTracerProvider(context.Background(),
		config.App{Name: "svc", Environment: config.EnvProduction, Version: "1"},
		config.OTel{Enabled: true, Endpoint: "203.0.113.1:4317",
			Protocol: config.OTLPGRPC, Insecure: true, SamplingRatio: 1})
	if err != nil {
		t.Fatalf("NewTracerProvider() = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx)
	})

	ctx, span := tp.Tracer("test").Start(
		telemetry.ContextWithRequestID(context.Background(), "req-42"), "op")
	defer span.End()

	logger.InfoContext(ctx, "hello")

	line := buf.String()
	if !strings.Contains(line, `"request_id":"req-42"`) {
		t.Errorf("request id missing: %s", line)
	}
	if !strings.Contains(line, `"trace_id":"`+span.SpanContext().TraceID().String()+`"`) {
		t.Errorf("trace id missing: %s", line)
	}
}
