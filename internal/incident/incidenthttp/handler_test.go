package incidenthttp_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/auth/authhttp"
	"github.com/namezzy/goscaffoldforlevi/internal/incident"
	"github.com/namezzy/goscaffoldforlevi/internal/incident/incidenthttp"
	"github.com/namezzy/goscaffoldforlevi/internal/openapitest"
)

// fakeRepo is an in-memory incident.Repository. It exists so the handler's
// routing, status codes and response shapes can be tested without a
// database; the state machine itself is covered by incident's service tests.
type fakeRepo struct {
	byID map[string]incident.Incident
}

func newFakeRepo() *fakeRepo { return &fakeRepo{byID: map[string]incident.Incident{}} }

func (f *fakeRepo) put(inc incident.Incident) incident.Incident {
	inc.UpdatedAt = time.Now()
	f.byID[inc.ID] = inc
	return inc
}

func (f *fakeRepo) ListFiltered(_ context.Context, filter incident.ListFilter, limit, offset int32) ([]incident.Incident, error) {
	var out []incident.Incident
	for _, inc := range f.byID {
		if filter.Status != nil && inc.Status != *filter.Status {
			continue
		}
		out = append(out, inc)
	}
	start := min(int(offset), len(out))
	return out[start:min(start+int(limit), len(out))], nil
}

func (f *fakeRepo) CountFiltered(ctx context.Context, filter incident.ListFilter) (int64, error) {
	items, _ := f.ListFiltered(ctx, filter, int32(len(f.byID)), 0)
	return int64(len(items)), nil
}

func (f *fakeRepo) CountOpenBySeverity(context.Context) (map[incident.Severity]int64, error) {
	out := map[incident.Severity]int64{}
	for _, inc := range f.byID {
		if inc.Status != incident.StatusResolved {
			out[inc.Severity]++
		}
	}
	return out, nil
}

func (f *fakeRepo) Get(_ context.Context, id string) (incident.Incident, error) {
	inc, ok := f.byID[id]
	if !ok {
		return incident.Incident{}, apperr.NotFound("INCIDENT_NOT_FOUND", "incident not found")
	}
	return inc, nil
}

func (f *fakeRepo) GetForUpdate(ctx context.Context, id string) (incident.Incident, error) {
	return f.Get(ctx, id)
}

func (f *fakeRepo) SetSeverity(ctx context.Context, id string, severity incident.Severity) (incident.Incident, error) {
	inc, err := f.Get(ctx, id)
	if err != nil {
		return incident.Incident{}, err
	}
	inc.Severity = severity
	return f.put(inc), nil
}

func (f *fakeRepo) AssignOwner(ctx context.Context, id string, ownerID *string) (incident.Incident, error) {
	inc, err := f.Get(ctx, id)
	if err != nil {
		return incident.Incident{}, err
	}
	inc.OwnerID = ownerID
	return f.put(inc), nil
}

func (f *fakeRepo) Acknowledge(ctx context.Context, id, actorID string) (incident.Incident, bool, error) {
	inc, err := f.Get(ctx, id)
	if err != nil {
		return incident.Incident{}, false, err
	}
	if inc.Status != incident.StatusOpen {
		return incident.Incident{}, false, nil
	}
	now := time.Now()
	inc.Status, inc.AcknowledgedAt, inc.AcknowledgedBy = incident.StatusAcknowledged, &now, &actorID
	return f.put(inc), true, nil
}

func (f *fakeRepo) Resolve(ctx context.Context, id, actorID string) (incident.Incident, bool, error) {
	inc, err := f.Get(ctx, id)
	if err != nil {
		return incident.Incident{}, false, err
	}
	if inc.Status == incident.StatusResolved {
		return incident.Incident{}, false, nil
	}
	now := time.Now()
	inc.Status, inc.ResolvedAt, inc.ResolvedBy = incident.StatusResolved, &now, &actorID
	return f.put(inc), true, nil
}

func (f *fakeRepo) RequestReplay(ctx context.Context, id, replayEventID string) (incident.Incident, error) {
	inc, err := f.Get(ctx, id)
	if err != nil {
		return incident.Incident{}, err
	}
	now := time.Now()
	pending := incident.ReplayPending
	inc.ReplayStatus, inc.ReplayRequestedAt, inc.ReplayResultAt = &pending, &now, nil
	inc.ReplayEventID = &replayEventID
	return f.put(inc), nil
}

type rollbackTx struct{ repo *fakeRepo }

func (tx rollbackTx) WithTx(ctx context.Context, fn func(context.Context) error) error {
	snapshot := maps.Clone(tx.repo.byID)
	if err := fn(ctx); err != nil {
		tx.repo.byID = snapshot
		return err
	}
	return nil
}

// fakeAudit both records entries and serves them back as activity, so the
// activity endpoint returns what the actions actually wrote.
type fakeAudit struct{ records []audit.Record }

func (a *fakeAudit) Record(_ context.Context, e audit.Entry) error {
	after, err := json.Marshal(e.After)
	if err != nil {
		return err
	}
	a.records = append(a.records, audit.Record{
		ID: uuid.Must(uuid.NewV7()).String(), ActorID: e.ActorID, Action: e.Action,
		TargetType: e.TargetType, TargetID: e.TargetID, Result: e.Result,
		After: after, CreatedAt: time.Now(),
	})
	return nil
}

func (a *fakeAudit) List(_ context.Context, f audit.Filter, _, _ int32) ([]audit.Record, error) {
	var out []audit.Record
	for _, r := range a.records {
		if f.TargetID == nil || r.TargetID == *f.TargetID {
			out = append(out, r)
		}
	}
	return out, nil
}

type fakeReplay struct{ replayed []uuid.UUID }

func (r *fakeReplay) ReplayDeadLetter(_ context.Context, id uuid.UUID) error {
	r.replayed = append(r.replayed, id)
	return nil
}

type fixture struct {
	srv    http.Handler
	repo   *fakeRepo
	replay *fakeReplay
}

func newFixture(t *testing.T) fixture {
	t.Helper()

	repo := newFakeRepo()
	trail := &fakeAudit{}
	replay := &fakeReplay{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := incidenthttp.NewHandler(incident.NewService(repo, rollbackTx{repo: repo}, trail, trail, replay), log)

	r := chi.NewRouter()
	r.Use(principalFromHeader)
	r.Route("/incidents", h.Routes(func(p auth.Permission) func(http.Handler) http.Handler {
		return authhttp.RequirePermission(p, log)
	}))
	return fixture{srv: r, repo: repo, replay: replay}
}

func (f fixture) seed(status incident.Status) incident.Incident {
	now := time.Now()
	return f.repo.put(incident.Incident{
		ID: incident.NewID(), Fingerprint: "welcome-email:user.created.v1", Severity: incident.SeverityWarning,
		Status: status, DeadLetterID: uuid.Must(uuid.NewV7()).String(), EventID: uuid.Must(uuid.NewV7()).String(),
		EventType: "user.created.v1", Consumer: "welcome-email", LastError: "smtp refused", OccurrenceCount: 1,
		FirstSeenAt: now, LastSeenAt: now, CreatedAt: now,
	})
}

// principalFromHeader mirrors messaginghttp's test helper: "X-Test-Principal:
// <uuid>/<role>", absent for the unauthenticated case.
func principalFromHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("X-Test-Principal")
		if raw == "" {
			next.ServeHTTP(w, r)
			return
		}
		id, role, _ := strings.Cut(raw, "/")
		ctx := auth.ContextWithPrincipal(r.Context(), auth.Principal{UserID: id, Role: auth.Role(role)})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

const (
	operator = "01930f8e-0000-7000-8000-000000000001/operator"
	auditor  = "01930f8e-0000-7000-8000-000000000002/auditor"
	readOnly = "01930f8e-0000-7000-8000-000000000003/read_only"
)

func do(t *testing.T, srv http.Handler, method, path, principal, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if principal != "" {
		req.Header.Set("X-Test-Principal", principal)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v: %s", err, rec.Body)
	}
	return body.Error.Code
}

func TestReadRoutesRequireIncidentRead(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	inc := f.seed(incident.StatusOpen)

	for _, path := range []string{"/incidents", "/incidents/summary", "/incidents/" + inc.ID, "/incidents/" + inc.ID + "/activity"} {
		cases := []struct {
			principal string
			want      int
		}{
			{"", http.StatusUnauthorized},
			{readOnly, http.StatusForbidden},
			{auditor, http.StatusOK},
			{operator, http.StatusOK},
		}
		for _, tc := range cases {
			if rec := do(t, f.srv, http.MethodGet, path, tc.principal, ""); rec.Code != tc.want {
				t.Errorf("GET %s as %q: status = %d, want %d: %s", path, tc.principal, rec.Code, tc.want, rec.Body)
			}
		}
	}
}

// TestActionsRequireIncidentManage: the route group only demands
// incident:read, so an auditor reaches every POST handler. This guards that
// the service's own incident:manage check still turns that into a 403 and
// nothing is changed or replayed.
func TestActionsRequireIncidentManage(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	inc := f.seed(incident.StatusOpen)

	actions := map[string]string{
		"severity":    `{"severity":"critical"}`,
		"owner":       `{"owner_id":"01930f8e-0000-7000-8000-000000000002"}`,
		"acknowledge": "",
		"notes":       `{"note":"looking"}`,
		"resolve":     "",
		"replay":      "",
	}
	for action, body := range actions {
		rec := do(t, f.srv, http.MethodPost, "/incidents/"+inc.ID+"/"+action, auditor, body)
		if rec.Code != http.StatusForbidden {
			t.Errorf("POST %s as auditor: status = %d, want 403: %s", action, rec.Code, rec.Body)
		}
	}
	if after := f.repo.byID[inc.ID]; after.Status != incident.StatusOpen || after.Severity != incident.SeverityWarning || after.OwnerID != nil {
		t.Errorf("incident changed by refused actions: %+v", after)
	}
	if len(f.replay.replayed) != 0 {
		t.Errorf("replayed = %v, want none", f.replay.replayed)
	}
}

func TestActionLifecycleStatusCodes(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	inc := f.seed(incident.StatusOpen)
	base := "/incidents/" + inc.ID

	steps := []struct {
		action, body string
		want         int
	}{
		{"severity", `{"severity":"critical"}`, http.StatusOK},
		{"owner", `{"owner_id":"01930f8e-0000-7000-8000-000000000001"}`, http.StatusOK},
		{"acknowledge", "", http.StatusOK},
		{"acknowledge", "", http.StatusConflict},
		{"notes", `{"note":"smtp provider is down"}`, http.StatusOK},
		{"replay", "", http.StatusAccepted},
		{"resolve", "", http.StatusOK},
	}
	for _, s := range steps {
		if rec := do(t, f.srv, http.MethodPost, base+"/"+s.action, operator, s.body); rec.Code != s.want {
			t.Fatalf("POST %s: status = %d, want %d: %s", s.action, rec.Code, s.want, rec.Body)
		}
	}

	got := f.repo.byID[inc.ID]
	if got.Severity != incident.SeverityCritical || got.OwnerID == nil || got.Status != incident.StatusResolved {
		t.Errorf("incident after lifecycle = %+v", got)
	}
	if len(f.replay.replayed) != 1 || f.replay.replayed[0].String() != inc.DeadLetterID {
		t.Errorf("replayed = %v, want exactly the incident's dead letter %s", f.replay.replayed, inc.DeadLetterID)
	}
}

func TestReplayRespondsPendingNotSuccess(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	inc := f.seed(incident.StatusAcknowledged)

	rec := do(t, f.srv, http.MethodPost, "/incidents/"+inc.ID+"/replay", operator, "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body)
	}
	var body struct {
		ReplayStatus *string `json:"replay_status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ReplayStatus == nil || *body.ReplayStatus != "pending" {
		t.Errorf("replay_status = %v, want \"pending\"", body.ReplayStatus)
	}
}

// TestResolvedIncidentActionsAre409 checks the HTTP mapping of the
// service's resolved-is-terminal rule, as documented in api/openapi.yaml.
func TestResolvedIncidentActionsAre409(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	inc := f.seed(incident.StatusResolved)

	actions := map[string]string{
		"severity":    `{"severity":"critical"}`,
		"owner":       `{"owner_id":"01930f8e-0000-7000-8000-000000000001"}`,
		"acknowledge": "",
		"notes":       `{"note":"too late"}`,
		"resolve":     "",
		"replay":      "",
	}
	for action, body := range actions {
		rec := do(t, f.srv, http.MethodPost, "/incidents/"+inc.ID+"/"+action, operator, body)
		if rec.Code != http.StatusConflict {
			t.Errorf("POST %s on resolved: status = %d, want 409: %s", action, rec.Code, rec.Body)
			continue
		}
		if code := errorCode(t, rec); code != "INCIDENT_INVALID_TRANSITION" {
			t.Errorf("POST %s on resolved: code = %q, want INCIDENT_INVALID_TRANSITION", action, code)
		}
	}
	if len(f.replay.replayed) != 0 {
		t.Errorf("replayed = %v, want none", f.replay.replayed)
	}
}

func TestBadRequestsAndMissingIncidents(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	inc := f.seed(incident.StatusOpen)
	base := "/incidents/" + inc.ID

	cases := []struct {
		name, method, path, body string
		want                     int
	}{
		{"malformed JSON", http.MethodPost, base + "/severity", `{"severity":`, http.StatusBadRequest},
		{"undefined severity", http.MethodPost, base + "/severity", `{"severity":"apocalyptic"}`, http.StatusBadRequest},
		{"empty note", http.MethodPost, base + "/notes", `{"note":"   "}`, http.StatusBadRequest},
		{"non-integer page", http.MethodGet, "/incidents?page=abc", "", http.StatusBadRequest},
		{"undefined status filter", http.MethodGet, "/incidents?status=bogus", "", http.StatusBadRequest},
		{"unknown incident", http.MethodGet, "/incidents/" + incident.NewID(), "", http.StatusNotFound},
		{"unknown incident replay", http.MethodPost, "/incidents/" + incident.NewID() + "/replay", "", http.StatusNotFound},
	}
	for _, tc := range cases {
		if rec := do(t, f.srv, tc.method, tc.path, operator, tc.body); rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d: %s", tc.name, rec.Code, tc.want, rec.Body)
		}
	}
	if after := f.repo.byID[inc.ID]; after.Severity != incident.SeverityWarning {
		t.Errorf("severity = %s after rejected requests, want warning", after.Severity)
	}
}

func TestListReturnsAnEmptyArrayNotNull(t *testing.T) {
	t.Parallel()

	rec := do(t, newFixture(t).srv, http.MethodGet, "/incidents", operator, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Errorf("empty list should serialise items as []: %s", rec.Body)
	}
}

// TestResponseBodiesMatchTheDocumentedSchemas compares real handler output
// against api/openapi.yaml, the same check messaginghttp and userhttp run.
func TestResponseBodiesMatchTheDocumentedSchemas(t *testing.T) {
	t.Parallel()

	spec := openapitest.Load(t)
	f := newFixture(t)
	inc := f.seed(incident.StatusOpen)
	f.seed(incident.StatusResolved)
	base := "/incidents/" + inc.ID

	expect := func(rec *httptest.ResponseRecorder, want int) []byte {
		t.Helper()
		if rec.Code != want {
			t.Fatalf("status = %d, want %d: %s", rec.Code, want, rec.Body)
		}
		return rec.Body.Bytes()
	}

	// Populate every optional field the actions set before reading back.
	expect(do(t, f.srv, http.MethodPost, base+"/owner", operator, `{"owner_id":"01930f8e-0000-7000-8000-000000000001"}`), http.StatusOK)
	expect(do(t, f.srv, http.MethodPost, base+"/acknowledge", operator, ""), http.StatusOK)
	expect(do(t, f.srv, http.MethodPost, base+"/notes", operator, `{"note":"checking"}`), http.StatusOK)
	spec.AssertMatches(t, "Incident", expect(do(t, f.srv, http.MethodPost, base+"/replay", operator, ""), http.StatusAccepted))
	spec.AssertMatches(t, "Incident", expect(do(t, f.srv, http.MethodPost, base+"/resolve", operator, ""), http.StatusOK))

	spec.AssertMatches(t, "Incident", expect(do(t, f.srv, http.MethodGet, base, operator, ""), http.StatusOK))
	spec.AssertMatches(t, "IncidentPage", expect(do(t, f.srv, http.MethodGet, "/incidents", operator, ""), http.StatusOK))
	spec.AssertMatches(t, "IncidentSummary", expect(do(t, f.srv, http.MethodGet, "/incidents/summary", operator, ""), http.StatusOK))

	var activity []json.RawMessage
	if err := json.Unmarshal(expect(do(t, f.srv, http.MethodGet, base+"/activity", operator, ""), http.StatusOK), &activity); err != nil {
		t.Fatalf("decode activity: %v", err)
	}
	if len(activity) != 5 {
		t.Errorf("activity = %d records, want 5 (owner, acknowledge, note, replay, resolve)", len(activity))
	}
	for _, record := range activity {
		spec.AssertMatches(t, "AuditEvent", record)
	}

	spec.AssertMatches(t, "Error", expect(do(t, f.srv, http.MethodPost, base+"/replay", operator, ""), http.StatusConflict))
}
