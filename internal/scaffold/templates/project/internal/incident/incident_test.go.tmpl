package incident_test

import (
	"context"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/incident"
)

// fakeRepository is an in-memory stand-in keyed by incident id, matching the
// hand-written style used across this codebase's other service tests (see
// ticket_test.go's fakeRepository).
type fakeRepository struct {
	byID map[string]incident.Incident
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{byID: map[string]incident.Incident{}}
}

func (f *fakeRepository) put(inc incident.Incident) incident.Incident {
	f.byID[inc.ID] = inc
	return inc
}

func (f *fakeRepository) ListFiltered(_ context.Context, filter incident.ListFilter, limit, offset int32) ([]incident.Incident, error) {
	var matched []incident.Incident
	for _, inc := range f.byID {
		if matches(inc, filter) {
			matched = append(matched, inc)
		}
	}
	start := int(offset)
	if start > len(matched) {
		start = len(matched)
	}
	end := start + int(limit)
	if end > len(matched) {
		end = len(matched)
	}
	return matched[start:end], nil
}

func (f *fakeRepository) CountFiltered(_ context.Context, filter incident.ListFilter) (int64, error) {
	var n int64
	for _, inc := range f.byID {
		if matches(inc, filter) {
			n++
		}
	}
	return n, nil
}

func matches(inc incident.Incident, filter incident.ListFilter) bool {
	if filter.Status != nil && inc.Status != *filter.Status {
		return false
	}
	if filter.Severity != nil && inc.Severity != *filter.Severity {
		return false
	}
	if filter.OwnerID != nil && (inc.OwnerID == nil || *inc.OwnerID != *filter.OwnerID) {
		return false
	}
	return true
}

func (f *fakeRepository) CountOpenBySeverity(context.Context) (map[incident.Severity]int64, error) {
	out := map[incident.Severity]int64{}
	for _, inc := range f.byID {
		if inc.Status != incident.StatusResolved {
			out[inc.Severity]++
		}
	}
	return out, nil
}

func errIncidentNotFound() error {
	return apperr.NotFound("INCIDENT_NOT_FOUND", "incident not found")
}

func (f *fakeRepository) Get(_ context.Context, id string) (incident.Incident, error) {
	inc, ok := f.byID[id]
	if !ok {
		return incident.Incident{}, errIncidentNotFound()
	}
	return inc, nil
}

func (f *fakeRepository) GetForUpdate(ctx context.Context, id string) (incident.Incident, error) {
	return f.Get(ctx, id)
}

func (f *fakeRepository) SetSeverity(_ context.Context, id string, severity incident.Severity) (incident.Incident, error) {
	inc, ok := f.byID[id]
	if !ok {
		return incident.Incident{}, errIncidentNotFound()
	}
	inc.Severity = severity
	return f.put(inc), nil
}

func (f *fakeRepository) AssignOwner(_ context.Context, id string, ownerID *string) (incident.Incident, error) {
	inc, ok := f.byID[id]
	if !ok {
		return incident.Incident{}, errIncidentNotFound()
	}
	inc.OwnerID = ownerID
	return f.put(inc), nil
}

func (f *fakeRepository) Acknowledge(_ context.Context, id, actorID string) (incident.Incident, bool, error) {
	inc, ok := f.byID[id]
	if !ok {
		return incident.Incident{}, false, errIncidentNotFound()
	}
	if inc.Status != incident.StatusOpen {
		return incident.Incident{}, false, nil
	}
	now := time.Now()
	inc.Status = incident.StatusAcknowledged
	inc.AcknowledgedAt = &now
	inc.AcknowledgedBy = &actorID
	return f.put(inc), true, nil
}

func (f *fakeRepository) Resolve(_ context.Context, id, actorID string) (incident.Incident, bool, error) {
	inc, ok := f.byID[id]
	if !ok {
		return incident.Incident{}, false, errIncidentNotFound()
	}
	if inc.Status == incident.StatusResolved {
		return incident.Incident{}, false, nil
	}
	now := time.Now()
	inc.Status = incident.StatusResolved
	inc.ResolvedAt = &now
	inc.ResolvedBy = &actorID
	return f.put(inc), true, nil
}

func (f *fakeRepository) RequestReplay(_ context.Context, id, replayEventID string) (incident.Incident, error) {
	inc, ok := f.byID[id]
	if !ok {
		return incident.Incident{}, errIncidentNotFound()
	}
	now := time.Now()
	pending := incident.ReplayPending
	inc.ReplayStatus = &pending
	inc.ReplayRequestedAt = &now
	inc.ReplayEventID = &replayEventID
	inc.ReplayResultAt = nil
	return f.put(inc), nil
}

// rollbackTx snapshots the fake repository before running fn and restores it
// if fn fails, matching ticket_test.go's rollbackTx.
type rollbackTx struct{ repo *fakeRepository }

func (tx rollbackTx) WithTx(ctx context.Context, fn func(context.Context) error) error {
	snapshot := maps.Clone(tx.repo.byID)
	if err := fn(ctx); err != nil {
		tx.repo.byID = snapshot
		return err
	}
	return nil
}

type nopAuditRecorder struct{ entries []audit.Entry }

func (r *nopAuditRecorder) Record(_ context.Context, e audit.Entry) error {
	r.entries = append(r.entries, e)
	return nil
}

type fakeActivityReader struct {
	records []audit.Record
	err     error
}

func (f *fakeActivityReader) List(context.Context, audit.Filter, int32, int32) ([]audit.Record, error) {
	return f.records, f.err
}

type fakeReplayTrigger struct {
	replayed []uuid.UUID
	err      error
}

func (f *fakeReplayTrigger) ReplayDeadLetter(_ context.Context, id uuid.UUID) error {
	if f.err != nil {
		return f.err
	}
	f.replayed = append(f.replayed, id)
	return nil
}

func newService(repo *fakeRepository) (*incident.Service, *nopAuditRecorder, *fakeReplayTrigger) {
	rec := &nopAuditRecorder{}
	replay := &fakeReplayTrigger{}
	svc := incident.NewService(repo, rollbackTx{repo: repo}, rec, &fakeActivityReader{}, replay)
	return svc, rec, replay
}

func managerContext(id string) context.Context {
	return auth.ContextWithPrincipal(context.Background(), auth.Principal{UserID: id, Role: auth.RoleOperator})
}

func auditorContext(id string) context.Context {
	return auth.ContextWithPrincipal(context.Background(), auth.Principal{UserID: id, Role: auth.RoleAuditor})
}

func readOnlyContext(id string) context.Context {
	return auth.ContextWithPrincipal(context.Background(), auth.Principal{UserID: id, Role: auth.RoleReadOnly})
}

func seedIncident(repo *fakeRepository, status incident.Status) incident.Incident {
	deadLetterID := uuid.Must(uuid.NewV7()).String()
	eventID := uuid.Must(uuid.NewV7()).String()
	return repo.put(incident.Incident{
		ID: incident.NewID(), Fingerprint: "worker:user.created.v1", Severity: incident.SeverityWarning,
		Status: status, DeadLetterID: deadLetterID, EventID: eventID, EventType: "user.created.v1",
		Consumer: "worker", LastError: "boom", OccurrenceCount: 1,
		FirstSeenAt: time.Now(), LastSeenAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
}

func TestListRequiresIncidentRead(t *testing.T) {
	t.Parallel()
	svc, _, _ := newService(newFakeRepository())

	_, err := svc.List(readOnlyContext("u1"), incident.ListFilter{}, 1, 20)
	if !apperr.Is(err, apperr.KindForbidden) {
		t.Fatalf("List() kind = %v, want forbidden", apperr.KindOf(err))
	}

	if _, err := svc.List(auditorContext("u1"), incident.ListFilter{}, 1, 20); err != nil {
		t.Fatalf("auditor List() = %v, want nil (auditor holds incident:read)", err)
	}
}

func TestMutatingActionsRequireIncidentManage(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	inc := seedIncident(repo, incident.StatusOpen)
	svc, _, _ := newService(repo)

	ctx := auditorContext("u1") // holds incident:read, not incident:manage

	if _, err := svc.SetSeverity(ctx, inc.ID, incident.SeverityCritical); !apperr.Is(err, apperr.KindForbidden) {
		t.Errorf("SetSeverity() kind = %v, want forbidden", apperr.KindOf(err))
	}
	if _, err := svc.AssignOwner(ctx, inc.ID, "someone"); !apperr.Is(err, apperr.KindForbidden) {
		t.Errorf("AssignOwner() kind = %v, want forbidden", apperr.KindOf(err))
	}
	if _, err := svc.Acknowledge(ctx, inc.ID); !apperr.Is(err, apperr.KindForbidden) {
		t.Errorf("Acknowledge() kind = %v, want forbidden", apperr.KindOf(err))
	}
	if _, err := svc.ProcessNote(ctx, inc.ID, "note"); !apperr.Is(err, apperr.KindForbidden) {
		t.Errorf("ProcessNote() kind = %v, want forbidden", apperr.KindOf(err))
	}
	if _, err := svc.Resolve(ctx, inc.ID); !apperr.Is(err, apperr.KindForbidden) {
		t.Errorf("Resolve() kind = %v, want forbidden", apperr.KindOf(err))
	}
	if _, err := svc.RequestReplay(ctx, inc.ID); !apperr.Is(err, apperr.KindForbidden) {
		t.Errorf("RequestReplay() kind = %v, want forbidden", apperr.KindOf(err))
	}
}

func TestAcknowledgeMovesOpenToAcknowledgedAndRecordsAudit(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	inc := seedIncident(repo, incident.StatusOpen)
	svc, rec, _ := newService(repo)

	updated, err := svc.Acknowledge(managerContext("op1"), inc.ID)
	if err != nil {
		t.Fatalf("Acknowledge() = %v", err)
	}
	if updated.Status != incident.StatusAcknowledged {
		t.Errorf("status = %v, want acknowledged", updated.Status)
	}
	if len(rec.entries) != 1 || rec.entries[0].Action != "incident.acknowledged" {
		t.Errorf("audit entries = %+v, want one incident.acknowledged", rec.entries)
	}
}

func TestAcknowledgeRefusesAnAlreadyAcknowledgedIncident(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	inc := seedIncident(repo, incident.StatusAcknowledged)
	svc, _, _ := newService(repo)

	_, err := svc.Acknowledge(managerContext("op1"), inc.ID)
	if !apperr.Is(err, apperr.KindConflict) {
		t.Fatalf("Acknowledge() kind = %v, want conflict", apperr.KindOf(err))
	}
}

func TestResolveRefusesAnAlreadyResolvedIncident(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	inc := seedIncident(repo, incident.StatusResolved)
	svc, _, _ := newService(repo)

	_, err := svc.Resolve(managerContext("op1"), inc.ID)
	if !apperr.Is(err, apperr.KindConflict) {
		t.Fatalf("Resolve() kind = %v, want conflict", apperr.KindOf(err))
	}
}

func TestRequestReplaySubmitsTheUnderlyingDeadLetterAndMarksPending(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	inc := seedIncident(repo, incident.StatusOpen)
	svc, _, replay := newService(repo)

	updated, err := svc.RequestReplay(managerContext("op1"), inc.ID)
	if err != nil {
		t.Fatalf("RequestReplay() = %v", err)
	}
	if updated.ReplayStatus == nil || *updated.ReplayStatus != incident.ReplayPending {
		t.Fatalf("replay status = %v, want pending", updated.ReplayStatus)
	}
	if len(replay.replayed) != 1 || replay.replayed[0].String() != inc.DeadLetterID {
		t.Errorf("replayed = %v, want exactly [%s]", replay.replayed, inc.DeadLetterID)
	}
}

func TestRequestReplayRollsBackWhenTheUnderlyingReplayFails(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	inc := seedIncident(repo, incident.StatusOpen)
	svc, _, replay := newService(repo)
	replay.err = errors.New("broker unavailable")

	if _, err := svc.RequestReplay(managerContext("op1"), inc.ID); err == nil {
		t.Fatal("RequestReplay() = nil, want an error")
	}

	reloaded, _ := repo.Get(context.Background(), inc.ID)
	if reloaded.ReplayStatus != nil {
		t.Errorf("replay status = %v, want nil (rolled back)", reloaded.ReplayStatus)
	}
}

func TestProcessNoteRecordsAnAuditEntryWithoutChangingStatus(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	inc := seedIncident(repo, incident.StatusOpen)
	svc, rec, _ := newService(repo)

	updated, err := svc.ProcessNote(managerContext("op1"), inc.ID, "looking into it")
	if err != nil {
		t.Fatalf("ProcessNote() = %v", err)
	}
	if updated.Status != incident.StatusOpen {
		t.Errorf("status = %v, want unchanged (open)", updated.Status)
	}
	if len(rec.entries) != 1 || rec.entries[0].Action != "incident.note_added" {
		t.Errorf("audit entries = %+v, want one incident.note_added", rec.entries)
	}
}

// TestAResolvedIncidentRefusesFurtherTriage: resolved is terminal. Before,
// only Acknowledge and Resolve checked status, so a resolved incident could
// still be re-triaged, annotated and — worst — replayed, re-running a
// consumer against a problem the team had already closed. The frontend hid
// the buttons; the API did not.
func TestAResolvedIncidentRefusesFurtherTriage(t *testing.T) {
	t.Parallel()
	actions := map[string]func(*incident.Service, context.Context, string) error{
		"SetSeverity": func(s *incident.Service, ctx context.Context, id string) error {
			_, err := s.SetSeverity(ctx, id, incident.SeverityCritical)
			return err
		},
		"AssignOwner": func(s *incident.Service, ctx context.Context, id string) error {
			_, err := s.AssignOwner(ctx, id, "op1")
			return err
		},
		"ProcessNote": func(s *incident.Service, ctx context.Context, id string) error {
			_, err := s.ProcessNote(ctx, id, "late note")
			return err
		},
		"RequestReplay": func(s *incident.Service, ctx context.Context, id string) error {
			_, err := s.RequestReplay(ctx, id)
			return err
		},
	}
	for name, act := range actions {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := newFakeRepository()
			inc := seedIncident(repo, incident.StatusResolved)
			svc, rec, replay := newService(repo)

			err := act(svc, managerContext("op1"), inc.ID)
			if !apperr.Is(err, apperr.KindConflict) {
				t.Fatalf("%s() on a resolved incident: kind = %v, want conflict", name, apperr.KindOf(err))
			}
			if len(rec.entries) != 0 {
				t.Errorf("audit entries = %d, want 0 for a refused action", len(rec.entries))
			}
			if len(replay.replayed) != 0 {
				t.Errorf("dead letters replayed = %v, want none", replay.replayed)
			}
			if after, _ := repo.Get(context.Background(), inc.ID); after.Severity != inc.Severity || after.OwnerID != nil || after.ReplayStatus != nil {
				t.Errorf("incident changed: %+v", after)
			}
		})
	}
}

func TestProcessNoteRejectsAnEmptyNote(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	inc := seedIncident(repo, incident.StatusOpen)
	svc, _, _ := newService(repo)

	_, err := svc.ProcessNote(managerContext("op1"), inc.ID, "")
	if !apperr.Is(err, apperr.KindInvalidArgument) {
		t.Fatalf("ProcessNote() kind = %v, want invalid argument", apperr.KindOf(err))
	}
}

func TestGetActivityRequiresIncidentRead(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	inc := seedIncident(repo, incident.StatusOpen)
	svc, _, _ := newService(repo)

	if _, err := svc.GetActivity(readOnlyContext("u1"), inc.ID); !apperr.Is(err, apperr.KindForbidden) {
		t.Errorf("GetActivity() kind = %v, want forbidden", apperr.KindOf(err))
	}
	if _, err := svc.GetActivity(auditorContext("u1"), inc.ID); err != nil {
		t.Errorf("auditor GetActivity() = %v, want nil", err)
	}
}
