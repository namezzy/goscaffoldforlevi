package httpx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/example/go-enterprise/internal/config"
	"github.com/example/go-enterprise/internal/httpx"
	"github.com/example/go-enterprise/internal/telemetry"
)

// §5.13. One request carrying every kind of secret this application handles,
// run through the whole middleware stack, with the logs and the spans it
// produces searched for each of them.
//
// This is deliberately a single end-to-end sweep rather than a test per
// middleware. Redaction is not a property of any one component: a value is
// safe only if no layer writes it, and every per-layer test passes on the day
// a new layer starts logging the request. The sweep is also the only shape
// that keeps working when a middleware is added, because a new middleware
// joins the stack without anyone remembering to extend the test.
func TestNoSecretReachesTheLogsOrTheSpans(t *testing.T) {
	const (
		password     = "correct horse battery staple"
		accessToken  = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyLTEifQ.c2lnbmF0dXJl"
		refreshToken = "8f14e45fceea167a5a36dedd4bea2543f14e45fceea167a5a36dedd4bea2543a"
		cookieValue  = "session=abc123secret"
		apiKey       = "apikey-EXAMPLE-not-a-real-credential"
	)
	secrets := map[string]string{
		"password":      password,
		"access token":  accessToken,
		"refresh token": refreshToken,
		"cookie":        cookieValue,
		"api key":       apiKey,
	}

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	metrics := telemetry.NewMetrics()
	router := httpx.NewRouter(httpx.RouterDeps{
		Config:  config.Config{HTTP: config.HTTP{MaxBodyBytes: 1 << 20, RequestTimeout: 5_000_000_000}},
		Logger:  logger,
		Health:  httpx.NewHealth(logger, "test"),
		Tracer:  tp,
		Metrics: metrics,
		Register: func(r chi.Router) {
			r.Post("/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
				// A handler that reads the body, as the real one does, so the
				// body is not left unread and silently unlogged.
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				w.WriteHeader(http.StatusUnauthorized)
			})
		},
	})

	body, err := json.Marshal(map[string]string{
		"email": "victim@example.com", "password": password,
		"refresh_token": refreshToken,
	})
	if err != nil {
		t.Fatalf("Marshal() = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost,
		"/v1/auth/login?token="+accessToken+"&api_key="+apiKey, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Cookie", cookieValue)
	req.Header.Set("X-Api-Key", apiKey)

	router.ServeHTTP(httptest.NewRecorder(), req)

	// Force the spans out before reading them.
	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	logged := logs.String()
	if logged == "" {
		t.Fatal("the request produced no logs at all; this test would pass for the wrong reason")
	}
	spanText := spanHaystack(rec.Ended())
	if spanText == "" {
		t.Fatal("the request produced no spans at all; this test would pass for the wrong reason")
	}

	for name, secret := range secrets {
		if strings.Contains(logged, secret) {
			t.Errorf("the %s appears in the logs:\n%s", name, logged)
		}
		if strings.Contains(spanText, secret) {
			t.Errorf("the %s appears in a span:\n%s", name, spanText)
		}
	}
}

func spanHaystack(spans []sdktrace.ReadOnlySpan) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.Name())
		b.WriteString("\x00")
		b.WriteString(s.Status().Description)
		for _, kv := range s.Attributes() {
			b.WriteString("\x00")
			b.WriteString(string(kv.Key))
			b.WriteString("=")
			b.WriteString(kv.Value.String())
		}
		for _, ev := range s.Events() {
			b.WriteString("\x00")
			b.WriteString(ev.Name)
			for _, kv := range ev.Attributes {
				b.WriteString("\x00")
				b.WriteString(kv.Value.String())
			}
		}
	}
	return b.String()
}
