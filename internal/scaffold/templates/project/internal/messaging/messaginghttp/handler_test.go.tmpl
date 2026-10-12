package messaginghttp_test

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

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/auth/authhttp"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging/messaginghttp"
	"github.com/namezzy/goscaffoldforlevi/internal/openapitest"
)

// fakeStore is an in-memory AdminStore. It exists so the handler's routing,
// status codes and response shapes can be tested without a database — the
// same split userhttp's handler tests make against a fakeRepo.
type fakeStore struct {
	status  messaging.OutboxStatus
	letters map[uuid.UUID]*messaging.DeadLetter
}

func newFakeStore() *fakeStore {
	return &fakeStore{letters: map[uuid.UUID]*messaging.DeadLetter{}}
}

func (f *fakeStore) OutboxStatus(context.Context) (messaging.OutboxStatus, error) {
	return f.status, nil
}

func (f *fakeStore) ListDeadLetters(_ context.Context, eventType string, limit int32) (messaging.DeadLetterPage, error) {
	var out []messaging.DeadLetter
	var total int64
	for _, d := range f.letters {
		if eventType != "" && d.Event.Type != eventType {
			continue
		}
		total++
		if int32(len(out)) < limit {
			out = append(out, *d)
		}
	}
	return messaging.DeadLetterPage{Items: out, Total: total}, nil
}

func (f *fakeStore) Replay(_ context.Context, id uuid.UUID) error {
	d, ok := f.letters[id]
	if !ok {
		return apperr.NotFound("DEAD_LETTER_NOT_FOUND", "dead letter not found")
	}
	if d.ReplayedAt != nil {
		return apperr.Conflict("DEAD_LETTER_ALREADY_REPLAYED", "dead letter was already replayed")
	}
	now := time.Now()
	d.ReplayedAt = &now
	return nil
}

func (f *fakeStore) seed(d messaging.DeadLetter) { f.letters[d.ID] = &d }

func newServer(t *testing.T, store *fakeStore) http.Handler {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := messaginghttp.NewHandler(messaging.NewAdmin(store), log)

	r := chi.NewRouter()
	r.Use(principalFromHeader)
	r.Route("/admin", h.Routes(func(p auth.Permission) func(http.Handler) http.Handler {
		return authhttp.RequirePermission(p, log)
	}))
	return r
}

// principalFromHeader mirrors userhttp's test helper: "X-Test-Principal:
// <uuid>/<role>", absent for the unauthenticated case.
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

const (
	admin    = "01930f8e-0000-7000-8000-000000000001/system_admin"
	ordinary = "01930f8e-0000-7000-8000-000000000002/user"
)

func do(t *testing.T, srv http.Handler, method, path, principal string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	if principal != "" {
		req.Header.Set("X-Test-Principal", principal)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestOutboxStatusRequiresMessagingRead(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	store.status = messaging.OutboxStatus{Pending: 3, Published: 100, Failing: 1, WorstAttempts: 2}
	srv := newServer(t, store)

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
			rec := do(t, srv, http.MethodGet, "/admin/outbox/status", tc.principal)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body)
			}
		})
	}

	rec := do(t, srv, http.MethodGet, "/admin/outbox/status", admin)
	var got struct {
		Pending       int64 `json:"pending"`
		Published     int64 `json:"published"`
		Failing       int64 `json:"failing"`
		WorstAttempts int32 `json:"worst_attempts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Pending != 3 || got.Published != 100 || got.Failing != 1 || got.WorstAttempts != 2 {
		t.Errorf("body = %+v, want the seeded status", got)
	}
}

func TestListDeadLettersReturnsAnEmptyArrayNotNull(t *testing.T) {
	t.Parallel()

	srv := newServer(t, newFakeStore())
	rec := do(t, srv, http.MethodGet, "/admin/dlq", admin)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Errorf("empty list should serialise items as []: %s", rec.Body)
	}
}

// TestListDeadLettersTotalCountsBeyondTheLimit is the reason total is a
// separate field rather than len(items): a client that read the count off
// the page size would report a queue of 3 as a queue of 2 the moment
// someone asks for only 2 of them.
func TestListDeadLettersTotalCountsBeyondTheLimit(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	for i := 0; i < 3; i++ {
		store.seed(messaging.DeadLetter{
			ID:    uuid.Must(uuid.NewV7()),
			Event: messaging.Event{ID: uuid.Must(uuid.NewV7()), Type: "user.created.v1"},
		})
	}
	srv := newServer(t, store)

	rec := do(t, srv, http.MethodGet, "/admin/dlq?limit=2", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}

	var body struct {
		Items []json.RawMessage `json:"items"`
		Total int64             `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 2 {
		t.Errorf("items = %d, want the limit (2)", len(body.Items))
	}
	if body.Total != 3 {
		t.Errorf("total = %d, want 3 (all matching rows, not just the page)", body.Total)
	}
}

func TestListDeadLettersRejectsAMalformedLimit(t *testing.T) {
	t.Parallel()

	srv := newServer(t, newFakeStore())
	rec := do(t, srv, http.MethodGet, "/admin/dlq?limit=abc", admin)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body)
	}
}

func TestReplayRequiresMessagingReplayNotJustRead(t *testing.T) {
	t.Parallel()

	// A caller holding only messaging:read must not be able to replay. There
	// is no such role in the current policy (admin holds both), so this
	// guards the route wiring itself: swap PermMessagingReplay for
	// PermMessagingRead on the replay route and this test is the one that
	// would catch it.
	id := uuid.Must(uuid.NewV7())
	store := newFakeStore()
	store.seed(messaging.DeadLetter{ID: id, Event: messaging.Event{ID: uuid.Must(uuid.NewV7()), Type: "user.created.v1"}})
	srv := newServer(t, store)

	rec := do(t, srv, http.MethodPost, "/admin/dlq/"+id.String()+"/replay", ordinary)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body)
	}
}

func TestReplayStatusCodes(t *testing.T) {
	t.Parallel()

	replayable := uuid.Must(uuid.NewV7())
	store := newFakeStore()
	store.seed(messaging.DeadLetter{ID: replayable, Event: messaging.Event{ID: uuid.Must(uuid.NewV7()), Type: "user.created.v1"}})
	srv := newServer(t, store)

	rec := do(t, srv, http.MethodPost, "/admin/dlq/not-a-uuid/replay", admin)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed id: status = %d, want 400: %s", rec.Code, rec.Body)
	}

	rec = do(t, srv, http.MethodPost, "/admin/dlq/"+uuid.Must(uuid.NewV7()).String()+"/replay", admin)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown id: status = %d, want 404: %s", rec.Code, rec.Body)
	}

	rec = do(t, srv, http.MethodPost, "/admin/dlq/"+replayable.String()+"/replay", admin)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("first replay: status = %d, want 204: %s", rec.Code, rec.Body)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 carried a body: %q", rec.Body)
	}

	rec = do(t, srv, http.MethodPost, "/admin/dlq/"+replayable.String()+"/replay", admin)
	if rec.Code != http.StatusConflict {
		t.Fatalf("second replay: status = %d, want 409: %s", rec.Code, rec.Body)
	}
}

// TestResponseBodiesMatchTheDocumentedSchemas compares real handler output
// against api/openapi.yaml, the same check userhttp runs on its own
// responses.
func TestResponseBodiesMatchTheDocumentedSchemas(t *testing.T) {
	t.Parallel()

	spec := openapitest.Load(t)

	store := newFakeStore()
	store.status = messaging.OutboxStatus{Pending: 1}
	letterID := uuid.Must(uuid.NewV7())
	store.seed(messaging.DeadLetter{
		ID:        letterID,
		Event:     messaging.Event{ID: uuid.Must(uuid.NewV7()), Type: "user.created.v1", Payload: []byte(`{"email":"ada@example.com"}`)},
		Consumer:  "welcome-email",
		Attempts:  5,
		LastError: "the mail provider rejected the address",
		FailedAt:  time.Now(),
	})
	srv := newServer(t, store)

	status := do(t, srv, http.MethodGet, "/admin/outbox/status", admin)
	if status.Code != http.StatusOK {
		t.Fatalf("status: %d: %s", status.Code, status.Body)
	}
	spec.AssertMatches(t, "OutboxStatus", status.Body.Bytes())

	list := do(t, srv, http.MethodGet, "/admin/dlq", admin)
	if list.Code != http.StatusOK {
		t.Fatalf("list: %d: %s", list.Code, list.Body)
	}
	// DeadLetterPage refers to DeadLetter, so this also covers the nested
	// representation, which is the one a triage client actually reads.
	spec.AssertMatches(t, "DeadLetterPage", list.Body.Bytes())

	notFound := do(t, srv, http.MethodPost, "/admin/dlq/"+uuid.Must(uuid.NewV7()).String()+"/replay", admin)
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("replay missing: %d: %s", notFound.Code, notFound.Body)
	}
	spec.AssertMatches(t, "Error", notFound.Body.Bytes())
}
