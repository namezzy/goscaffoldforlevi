package requesttrackhttp_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/auth/authhttp"
	"github.com/namezzy/goscaffoldforlevi/internal/openapitest"
	"github.com/namezzy/goscaffoldforlevi/internal/requesttrack"
	"github.com/namezzy/goscaffoldforlevi/internal/requesttrack/requesttrackhttp"
)

const (
	admin    = "01930f8e-0000-7000-8000-000000000001/system_admin"
	ordinary = "01930f8e-0000-7000-8000-000000000002/user"
)

func newServer(t *testing.T, tracker *requesttrack.Tracker) http.Handler {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := requesttrackhttp.NewHandler(tracker, log)

	r := chi.NewRouter()
	r.Use(principalFromHeader)
	r.Route("/admin/requests", h.Routes(func(p auth.Permission) func(http.Handler) http.Handler {
		return authhttp.RequirePermission(p, log)
	}))
	return r
}

func principalFromHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("X-Test-Principal")
		if raw == "" {
			next.ServeHTTP(w, r)
			return
		}
		id, role, _ := strings.Cut(raw, "/")
		ctx := auth.ContextWithPrincipal(r.Context(), auth.Principal{
			UserID: id,
			Role:   auth.Role(role),
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func do(t *testing.T, srv http.Handler, principal string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/admin/requests", nil)
	if principal != "" {
		req.Header.Set("X-Test-Principal", principal)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestGetRequiresRequestsRead(t *testing.T) {
	t.Parallel()

	srv := newServer(t, requesttrack.NewTracker(10))

	cases := []struct {
		name      string
		principal string
		want      int
	}{
		{"no principal is 401", "", http.StatusUnauthorized},
		{"an ordinary user is 403", ordinary, http.StatusForbidden},
		{"an admin is 200", admin, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, srv, tc.principal)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body)
			}
		})
	}
}

func TestGetReturnsRecentEventsAsJSON(t *testing.T) {
	t.Parallel()

	tracker := requesttrack.NewTracker(10)
	tracker.Record(requesttrack.Event{
		Time: time.Now(), Method: "GET", Path: "/api/v1/users", Status: 200,
		Bytes: 42, DurationMS: 1.5, RequestID: "req-1",
	})
	srv := newServer(t, tracker)

	rec := do(t, srv, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body struct {
		Items []struct {
			Path      string `json:"path"`
			RequestID string `json:"request_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].Path != "/api/v1/users" || body.Items[0].RequestID != "req-1" {
		t.Errorf("items = %+v, want one event for /api/v1/users", body.Items)
	}
}

func TestGetReturnsAnEmptyArrayNotNull(t *testing.T) {
	t.Parallel()

	srv := newServer(t, requesttrack.NewTracker(10))
	rec := do(t, srv, admin)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Errorf("empty snapshot should serialise items as []: %s", rec.Body)
	}
}

// TestStreamDeliversLiveEvents exercises the Accept: text/event-stream path
// end to end over a real connection — httptest.ResponseRecorder cannot
// stand in here, because the handler blocks on the request context and
// writes incrementally, and a Recorder only exposes what was written after
// the handler returns.
func TestStreamDeliversLiveEvents(t *testing.T) {
	t.Parallel()

	tracker := requesttrack.NewTracker(10)
	ts := httptest.NewServer(newServer(t, tracker))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/admin/requests", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("X-Test-Principal", admin)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}

	reader := bufio.NewReader(resp.Body)

	// Give the handler a moment to reach tracker.Subscribe() before the
	// event is recorded, or it is published to no one and never arrives.
	time.Sleep(50 * time.Millisecond)
	tracker.Record(requesttrack.Event{
		Time: time.Now(), Method: "POST", Path: "/api/v1/users", Status: 201,
		Bytes: 7, DurationMS: 3.25, RequestID: "req-live",
	})

	line, err := readDataLine(reader)
	if err != nil {
		t.Fatalf("reading the SSE stream: %v", err)
	}

	var got struct {
		Path      string `json:"path"`
		RequestID string `json:"request_id"`
		Status    int    `json:"status"`
	}
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("decode SSE payload %q: %v", line, err)
	}
	if got.Path != "/api/v1/users" || got.RequestID != "req-live" || got.Status != 201 {
		t.Errorf("event = %+v, want the recorded event", got)
	}
}

// TestResponseBodyMatchesTheDocumentedSchema compares real handler output
// against api/openapi.yaml, the same check every other handler package runs
// on its own responses.
func TestResponseBodyMatchesTheDocumentedSchema(t *testing.T) {
	t.Parallel()

	spec := openapitest.Load(t)

	tracker := requesttrack.NewTracker(10)
	tracker.Record(requesttrack.Event{
		Time: time.Now(), Method: "GET", Path: "/api/v1/users", Status: 200,
		Bytes: 42, DurationMS: 1.5, RequestID: "req-1",
		IP: "203.0.113.5", CountryCode: "CN", CountryName: "中国",
	})
	srv := newServer(t, tracker)

	rec := do(t, srv, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"country_name":"中国"`) {
		t.Fatalf("expected the geo fields in the response: %s", rec.Body)
	}
	// RequestEventPage refers to RequestEvent, so this also covers the
	// nested representation, which is the one a client actually reads.
	spec.AssertMatches(t, "RequestEventPage", rec.Body.Bytes())
}

// readDataLine skips SSE comment/heartbeat lines and returns the payload of
// the next "data: ..." line.
func readDataLine(r *bufio.Reader) (string, error) {
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		line = strings.TrimRight(line, "\r\n")
		if payload, ok := strings.CutPrefix(line, "data: "); ok {
			return payload, nil
		}
	}
}
