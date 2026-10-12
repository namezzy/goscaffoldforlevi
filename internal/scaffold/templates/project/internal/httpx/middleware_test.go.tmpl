package httpx

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/config"
	"github.com/namezzy/goscaffoldforlevi/internal/geoip"
	"github.com/namezzy/goscaffoldforlevi/internal/requesttrack"
	"github.com/namezzy/goscaffoldforlevi/internal/telemetry"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func captureLogger() (*slog.Logger, *strings.Builder) {
	var buf strings.Builder
	return slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})), &buf
}

func TestRequestIDGeneratesWhenAbsent(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = telemetry.RequestIDFromContext(r.Context())
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if _, err := uuid.Parse(seen); err != nil {
		t.Fatalf("context request id %q is not a uuid: %v", seen, err)
	}
	if got := rec.Header().Get(HeaderRequestID); got != seen {
		t.Fatalf("response header = %q, want %q", got, seen)
	}
}

// TestRequestIDRejectsUntrustedInput is a small but real defence: the header
// is client-controlled, and echoing it unvalidated into every log line invites
// log injection and unbounded field sizes.
func TestRequestIDRejectsUntrustedInput(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = telemetry.RequestIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderRequestID, "not-a-uuid\ninjected=\"line\"")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if strings.Contains(seen, "injected") {
		t.Fatalf("attacker-controlled value survived: %q", seen)
	}
	if _, err := uuid.Parse(seen); err != nil {
		t.Fatalf("replacement %q is not a uuid: %v", seen, err)
	}
}

func TestRequestIDHonoursAValidInboundID(t *testing.T) {
	want := uuid.NewString()

	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = telemetry.RequestIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderRequestID, want)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if seen != want {
		t.Fatalf("request id = %q, want the inbound %q", seen, want)
	}
}

func TestRecoveryTurnsPanicIntoA500(t *testing.T) {
	logger, logs := captureLogger()
	h := Recovery(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("database password is hunter2")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	// The panic value routinely contains internal detail; it belongs in the
	// log, never in the response.
	if strings.Contains(rec.Body.String(), "hunter2") {
		t.Fatalf("panic value leaked to the client: %s", rec.Body.String())
	}
	if !strings.Contains(logs.String(), "hunter2") {
		t.Fatal("panic value was not logged")
	}
	if !strings.Contains(logs.String(), "stack") {
		t.Fatal("stack trace was not logged")
	}
}

func TestBodyLimitRejectsOversizedBodies(t *testing.T) {
	h := BodyLimit(16)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("a", 128)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

func TestTimeoutReturns504WhenTheHandlerStalls(t *testing.T) {
	logger := discardLogger()
	released := make(chan struct{})
	t.Cleanup(func() { close(released) })

	h := Timeout(20*time.Millisecond, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		<-released
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504", rec.Code)
	}
}

// TestTimeoutCancelsTheHandlerContext is the property that makes this
// middleware worth having over http.TimeoutHandler: the deadline must reach
// the database call, not just the response writer.
func TestTimeoutCancelsTheHandlerContext(t *testing.T) {
	cancelled := make(chan struct{})

	h := Timeout(20*time.Millisecond, discardLogger())(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
			close(cancelled)
		}))

	go h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("handler context was never cancelled")
	}
}

func TestTimeoutLetsFastHandlersThrough(t *testing.T) {
	h := Timeout(time.Second, discardLogger())(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want 418", rec.Code)
	}
}

func TestCORS(t *testing.T) {
	cfg := config.HTTP{CORSAllowedOrigins: []string{"https://app.example.com"}}
	h := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("allowed origin is echoed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "https://app.example.com")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
			t.Fatalf("Allow-Origin = %q", got)
		}
		if got := rec.Header().Get("Vary"); !strings.Contains(got, "Origin") {
			t.Fatalf("Vary = %q, want it to contain Origin", got)
		}
	})

	t.Run("disallowed origin gets no header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "https://evil.example.com")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("Allow-Origin = %q, want empty", got)
		}
	})

	t.Run("preflight short-circuits", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "https://app.example.com")
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		got := rec.Header().Get("Access-Control-Allow-Methods")
		if got == "" {
			t.Fatal("Allow-Methods is missing")
		}
		// Every method an actual route uses must be listed, or a browser's
		// preflight rejects a request curl and the Bruno collection would
		// have let straight through — see the PUT regression this guards.
		for _, method := range []string{
			http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete,
		} {
			if !strings.Contains(got, method) {
				t.Errorf("Allow-Methods = %q, missing %s", got, method)
			}
		}
	})
}

func TestWriteErrorHidesInternalDetail(t *testing.T) {
	logger, logs := captureLogger()
	ctx := telemetry.ContextWithRequestID(context.Background(), "req-1")

	cause := apperr.Internal(errStubWithSecret{})
	rec := httptest.NewRecorder()
	WriteError(ctx, rec, logger, cause)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "users_email_key") {
		t.Fatalf("internal detail leaked: %s", body)
	}
	if !strings.Contains(logs.String(), "users_email_key") {
		t.Fatal("internal detail was not logged")
	}

	var parsed struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if parsed.Error.Code != apperr.CodeInternal || parsed.Error.Message != "internal error" {
		t.Fatalf("500 body is not the fixed form: %+v", parsed.Error)
	}
	if parsed.RequestID != "req-1" {
		t.Fatalf("request_id = %q, want req-1", parsed.RequestID)
	}
}

func TestWriteErrorPreservesClientFacingDetail(t *testing.T) {
	rec := httptest.NewRecorder()
	err := apperr.Invalid("VALIDATION_FAILED", "invalid request",
		apperr.Detail{Field: "email", Reason: "required"})

	WriteError(context.Background(), rec, discardLogger(), err)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"field":"email"`) {
		t.Fatalf("details were dropped: %s", rec.Body.String())
	}
}

// TestWriteJSONHasNoEnvelope pins ADR-0004: success responses carry the
// resource directly. Adding an envelope later would break every client.
func TestWriteJSONHasNoEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(context.Background(), rec, discardLogger(), http.StatusOK,
		map[string]string{"id": "1"})

	var parsed map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, wrapped := parsed["data"]; wrapped {
		t.Fatalf("success response is enveloped: %s", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestWriteJSONNilBodyWritesNoContent(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(context.Background(), rec, discardLogger(), http.StatusNoContent, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", rec.Body.String())
	}
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	// Silently ignoring a misspelled field turns a client bug into an
	// "the update did nothing" support ticket.
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"nmae":"typo"}`))

	var target struct {
		Name string `json:"name"`
	}
	err := DecodeJSON(req, &target)
	if err == nil {
		t.Fatal("DecodeJSON accepted an unknown field")
	}
	if apperr.KindOf(err) != apperr.KindInvalidArgument {
		t.Fatalf("Kind = %v, want KindInvalidArgument", apperr.KindOf(err))
	}
}

func TestDecodeJSONRejectsEmptyAndTrailingContent(t *testing.T) {
	var target struct {
		Name string `json:"name"`
	}

	empty := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	if err := DecodeJSON(empty, &target); err == nil {
		t.Fatal("DecodeJSON accepted an empty body")
	}

	two := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"a"}{"name":"b"}`))
	if err := DecodeJSON(two, &target); err == nil {
		t.Fatal("DecodeJSON accepted two documents")
	}
}

// TestTrackRecordsIPAndCountry is the reason Track takes a trustedProxies
// argument and a geoip.Resolver at all, rather than just the tracker: an
// event with no IP or an always-unresolved country would still exercise
// every other field this middleware sets.
func TestTrackRecordsIPAndCountry(t *testing.T) {
	t.Parallel()

	tracker := requesttrack.NewTracker(10)
	// No database configured — geoip.Open("", ...) still classifies a
	// private address, which a request from a test's loopback peer is.
	geo, err := geoip.Open("", "zh-CN")
	if err != nil {
		t.Fatalf("geoip.Open: %v", err)
	}

	h := Track(tracker, geo, 0)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/things", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	h.ServeHTTP(httptest.NewRecorder(), req)

	got := tracker.Recent()
	if len(got) != 1 {
		t.Fatalf("Recent() has %d events, want 1", len(got))
	}
	if got[0].IP != "203.0.113.7" {
		t.Errorf("IP = %q, want 203.0.113.7", got[0].IP)
	}
	// 203.0.113.0/24 is TEST-NET-3 (RFC 5737): documentation space, not
	// private, so this proves the address reached the resolver rather than
	// being short-circuited — the resolver just has no database to answer
	// from, so the country comes back unresolved.
	if got[0].CountryCode != "" {
		t.Errorf("CountryCode = %q, want empty with no database configured", got[0].CountryCode)
	}
}

func TestTrackRecordsAPrivateAddressAsSuch(t *testing.T) {
	t.Parallel()

	tracker := requesttrack.NewTracker(10)
	geo, err := geoip.Open("", "zh-CN")
	if err != nil {
		t.Fatalf("geoip.Open: %v", err)
	}

	h := Track(tracker, geo, 0)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/things", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	h.ServeHTTP(httptest.NewRecorder(), req)

	got := tracker.Recent()
	if len(got) != 1 || got[0].CountryName != "内网" {
		t.Fatalf("Recent() = %+v, want one event with CountryName 内网", got)
	}
}

// TestTrackWithoutAResolverStillRecordsTheIP is what makes a nil Geo a
// supported RouterDeps configuration rather than a crash waiting for
// whoever forgets to set it.
func TestTrackWithoutAResolverStillRecordsTheIP(t *testing.T) {
	t.Parallel()

	tracker := requesttrack.NewTracker(10)
	h := Track(tracker, nil, 0)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/things", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	h.ServeHTTP(httptest.NewRecorder(), req)

	got := tracker.Recent()
	if len(got) != 1 || got[0].IP != "203.0.113.7" || got[0].CountryName != "" {
		t.Fatalf("Recent() = %+v, want the IP recorded with no country", got)
	}
}

type errStubWithSecret struct{}

func (errStubWithSecret) Error() string {
	return `pq: duplicate key value violates unique constraint "users_email_key"`
}
