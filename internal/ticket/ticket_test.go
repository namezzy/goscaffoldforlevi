package ticket_test

import (
	"context"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/notification"
	"github.com/namezzy/goscaffoldforlevi/internal/ticket"
)

// fakeRepository is an in-memory stand-in keyed by ticket id, matching the
// hand-written style used across this codebase's other service tests (see
// invite_test.go's fakeRepository) — a real store, not a generated mock,
// so the authorization and version-conflict behaviour under test runs
// against something that actually stores and conditions on state.
type fakeRepository struct {
	byID map[string]ticket.Ticket
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{byID: map[string]ticket.Ticket{}}
}

func (f *fakeRepository) Create(_ context.Context, t ticket.Ticket) (ticket.Ticket, error) {
	t.Version = 1
	t.CreatedAt = time.Now()
	t.UpdatedAt = t.CreatedAt
	f.byID[t.ID] = t
	return t, nil
}

func (f *fakeRepository) GetByID(_ context.Context, id string) (ticket.Ticket, error) {
	t, ok := f.byID[id]
	if !ok {
		return ticket.Ticket{}, apperr.NotFound("TICKET_NOT_FOUND", "ticket not found")
	}
	return t, nil
}

func (f *fakeRepository) ListFiltered(_ context.Context, filter ticket.ListFilter, limit, offset int32) ([]ticket.Ticket, error) {
	var matched []ticket.Ticket
	for _, t := range f.byID {
		if matches(t, filter) {
			matched = append(matched, t)
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

func (f *fakeRepository) CountFiltered(_ context.Context, filter ticket.ListFilter) (int64, error) {
	var n int64
	for _, t := range f.byID {
		if matches(t, filter) {
			n++
		}
	}
	return n, nil
}

func matches(t ticket.Ticket, filter ticket.ListFilter) bool {
	if filter.RequesterID != nil && t.RequesterID != *filter.RequesterID {
		return false
	}
	if filter.AssigneeID != nil && (t.AssigneeID == nil || *t.AssigneeID != *filter.AssigneeID) {
		return false
	}
	if filter.Status != nil && t.Status != *filter.Status {
		return false
	}
	if filter.Priority != nil && t.Priority != *filter.Priority {
		return false
	}
	return true
}

func (f *fakeRepository) CountByStatus(_ context.Context) (map[ticket.Status]int64, error) {
	out := map[ticket.Status]int64{}
	for _, t := range f.byID {
		out[t.Status]++
	}
	return out, nil
}

func (f *fakeRepository) Update(_ context.Context, id string, expectedVersion int32, p ticket.Patch) (ticket.Ticket, error) {
	t, ok := f.byID[id]
	if !ok {
		return ticket.Ticket{}, apperr.NotFound("TICKET_NOT_FOUND", "ticket not found")
	}
	if t.Version != expectedVersion {
		return ticket.Ticket{}, apperr.Conflict("TICKET_VERSION_CONFLICT", "version mismatch")
	}
	if p.Status != nil {
		t.Status = *p.Status
		if t.Status == ticket.StatusClosed {
			now := time.Now()
			t.ClosedAt = &now
		}
	}
	if p.ClearAssignee {
		t.AssigneeID = nil
	} else if p.AssigneeID != nil {
		t.AssigneeID = p.AssigneeID
	}
	t.Version++
	t.UpdatedAt = time.Now()
	f.byID[id] = t
	return t, nil
}

// rollbackTx snapshots the fake repository before running fn and restores
// it if fn fails, matching the same property the real TxManager guarantees
// — see user/service_test.go's rollbackTx for the reasoning against a
// no-op fake.
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

type memoryNotifier struct {
	sent    []notification.NewNotification
	failErr error
}

func (m *memoryNotifier) Notify(_ context.Context, n notification.NewNotification) error {
	if m.failErr != nil {
		return m.failErr
	}
	m.sent = append(m.sent, n)
	return nil
}

func newService(repo *fakeRepository) (*ticket.Service, *nopAuditRecorder, *memoryNotifier) {
	audit := &nopAuditRecorder{}
	notify := &memoryNotifier{}
	svc := ticket.NewService(repo, rollbackTx{repo: repo}, audit, &fakeActivityReader{}, notify)
	return svc, audit, notify
}

func requesterContext(id string) context.Context {
	return auth.ContextWithPrincipal(context.Background(), auth.Principal{UserID: id, Role: auth.RoleOperator})
}

func managerContext(id string) context.Context {
	return auth.ContextWithPrincipal(context.Background(), auth.Principal{UserID: id, Role: auth.RoleMemberAdmin})
}

func auditorContext(id string) context.Context {
	return auth.ContextWithPrincipal(context.Background(), auth.Principal{UserID: id, Role: auth.RoleAuditor})
}

func createTicket(t *testing.T, svc *ticket.Service, requesterID string) ticket.Ticket {
	t.Helper()
	tk, err := svc.Create(requesterContext(requesterID), ticket.NewTicketInput{
		Title: "printer is broken", Description: "the office printer jams every page",
		Category: "facilities", Priority: ticket.PriorityNormal,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return tk
}

func TestCreateStartsInPending(t *testing.T) {
	svc, _, _ := newService(newFakeRepository())
	tk := createTicket(t, svc, "requester-1")
	if tk.Status != ticket.StatusPending {
		t.Fatalf("status = %s, want pending", tk.Status)
	}
	if tk.Version != 1 {
		t.Fatalf("version = %d, want 1", tk.Version)
	}
}

func TestAssignRequiresTicketManage(t *testing.T) {
	svc, _, _ := newService(newFakeRepository())
	tk := createTicket(t, svc, "requester-1")

	_, err := svc.Assign(requesterContext("requester-1"), tk.ID, "assignee-1", tk.Version)
	if !apperr.Is(err, apperr.KindForbidden) {
		t.Fatalf("err = %v, want forbidden", err)
	}
}

func TestAssignMovesPendingToInProgressAndNotifiesTheAssignee(t *testing.T) {
	repo := newFakeRepository()
	svc, _, notify := newService(repo)
	tk := createTicket(t, svc, "requester-1")

	updated, err := svc.Assign(managerContext("manager-1"), tk.ID, "assignee-1", tk.Version)
	if err != nil {
		t.Fatalf("assign: %v", err)
	}
	if updated.Status != ticket.StatusInProgress {
		t.Fatalf("status = %s, want in_progress", updated.Status)
	}
	if updated.AssigneeID == nil || *updated.AssigneeID != "assignee-1" {
		t.Fatalf("assignee = %v, want assignee-1", updated.AssigneeID)
	}
	if len(notify.sent) != 1 || notify.sent[0].UserID != "assignee-1" {
		t.Fatalf("notify.sent = %+v, want one notification to assignee-1", notify.sent)
	}
}

func TestAssignRejectsAnAlreadyAssignedTicket(t *testing.T) {
	repo := newFakeRepository()
	svc, _, _ := newService(repo)
	tk := createTicket(t, svc, "requester-1")
	updated, err := svc.Assign(managerContext("manager-1"), tk.ID, "assignee-1", tk.Version)
	if err != nil {
		t.Fatalf("assign: %v", err)
	}

	_, err = svc.Assign(managerContext("manager-1"), tk.ID, "assignee-2", updated.Version)
	if !apperr.Is(err, apperr.KindConflict) {
		t.Fatalf("err = %v, want conflict (invalid transition)", err)
	}
}

func TestAssignWithAStaleVersionIsAConflict(t *testing.T) {
	repo := newFakeRepository()
	svc, _, _ := newService(repo)
	tk := createTicket(t, svc, "requester-1")

	_, err := svc.Assign(managerContext("manager-1"), tk.ID, "assignee-1", tk.Version+1)
	if !apperr.Is(err, apperr.KindConflict) {
		t.Fatalf("err = %v, want conflict", err)
	}
}

func TestProcessNoteAllowsTheAssigneeButNotAnUnrelatedMember(t *testing.T) {
	repo := newFakeRepository()
	svc, _, _ := newService(repo)
	tk := createTicket(t, svc, "requester-1")
	tk, err := svc.Assign(managerContext("manager-1"), tk.ID, "assignee-1", tk.Version)
	if err != nil {
		t.Fatalf("assign: %v", err)
	}

	if _, err := svc.ProcessNote(requesterContext("someone-else"), tk.ID, "looked into it", tk.Version); !apperr.Is(err, apperr.KindForbidden) {
		t.Fatalf("err = %v, want forbidden", err)
	}

	updated, err := svc.ProcessNote(requesterContext("assignee-1"), tk.ID, "looked into it", tk.Version)
	if err != nil {
		t.Fatalf("process note as assignee: %v", err)
	}
	if updated.Version != tk.Version+1 {
		t.Fatalf("version = %d, want %d (note still bumps the optimistic-concurrency token)", updated.Version, tk.Version+1)
	}
}

func TestRequestInfoThenProvideInfoRoundTrips(t *testing.T) {
	repo := newFakeRepository()
	svc, _, notify := newService(repo)
	tk := createTicket(t, svc, "requester-1")
	tk, err := svc.Assign(managerContext("manager-1"), tk.ID, "assignee-1", tk.Version)
	if err != nil {
		t.Fatalf("assign: %v", err)
	}

	tk, err = svc.RequestInfo(requesterContext("assignee-1"), tk.ID, "which office is it in?", tk.Version)
	if err != nil {
		t.Fatalf("request info: %v", err)
	}
	if tk.Status != ticket.StatusNeedsInfo {
		t.Fatalf("status = %s, want needs_info", tk.Status)
	}

	// Only the requester may answer — the assignee themselves cannot.
	if _, err := svc.ProvideInfo(requesterContext("assignee-1"), tk.ID, "third floor", tk.Version); !apperr.Is(err, apperr.KindForbidden) {
		t.Fatalf("err = %v, want forbidden", err)
	}

	tk, err = svc.ProvideInfo(requesterContext("requester-1"), tk.ID, "third floor", tk.Version)
	if err != nil {
		t.Fatalf("provide info: %v", err)
	}
	if tk.Status != ticket.StatusInProgress {
		t.Fatalf("status = %s, want in_progress", tk.Status)
	}

	wantTypes := map[string]bool{"ticket_assigned": false, "ticket_needs_info": false}
	for _, n := range notify.sent {
		if _, ok := wantTypes[n.Type]; ok {
			wantTypes[n.Type] = true
		}
	}
	for typ, seen := range wantTypes {
		if !seen {
			t.Fatalf("notification type %q was never sent; sent=%+v", typ, notify.sent)
		}
	}
}

func TestCompleteRequiresInProgressAndNotifiesTheRequester(t *testing.T) {
	repo := newFakeRepository()
	svc, _, notify := newService(repo)
	tk := createTicket(t, svc, "requester-1")

	// Cannot complete straight from pending.
	if _, err := svc.Complete(managerContext("manager-1"), tk.ID, "", tk.Version); !apperr.Is(err, apperr.KindConflict) {
		t.Fatalf("err = %v, want conflict", err)
	}

	tk, err := svc.Assign(managerContext("manager-1"), tk.ID, "assignee-1", tk.Version)
	if err != nil {
		t.Fatalf("assign: %v", err)
	}
	tk, err = svc.Complete(requesterContext("assignee-1"), tk.ID, "fixed the jam", tk.Version)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if tk.Status != ticket.StatusCompleted {
		t.Fatalf("status = %s, want completed", tk.Status)
	}
	found := false
	for _, n := range notify.sent {
		if n.Type == "ticket_completed" && n.UserID == "requester-1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no ticket_completed notification to the requester; sent=%+v", notify.sent)
	}
}

func TestCloseByRequesterOnlyFromCompleted(t *testing.T) {
	repo := newFakeRepository()
	svc, _, _ := newService(repo)
	tk := createTicket(t, svc, "requester-1")
	tk, err := svc.Assign(managerContext("manager-1"), tk.ID, "assignee-1", tk.Version)
	if err != nil {
		t.Fatalf("assign: %v", err)
	}

	if _, err := svc.Close(requesterContext("requester-1"), tk.ID, tk.Version); !apperr.Is(err, apperr.KindConflict) {
		t.Fatalf("close from in_progress as requester: err = %v, want conflict", err)
	}

	tk, err = svc.Complete(requesterContext("assignee-1"), tk.ID, "", tk.Version)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	tk, err = svc.Close(requesterContext("requester-1"), tk.ID, tk.Version)
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if tk.Status != ticket.StatusClosed {
		t.Fatalf("status = %s, want closed", tk.Status)
	}
	if tk.ClosedAt == nil {
		t.Fatalf("closed_at not set")
	}
}

func TestCloseByManagerFromAnyOpenStatus(t *testing.T) {
	repo := newFakeRepository()
	svc, _, _ := newService(repo)
	tk := createTicket(t, svc, "requester-1")

	closed, err := svc.Close(managerContext("manager-1"), tk.ID, tk.Version)
	if err != nil {
		t.Fatalf("close from pending as manager: %v", err)
	}
	if closed.Status != ticket.StatusClosed {
		t.Fatalf("status = %s, want closed", closed.Status)
	}
}

func TestGetIsScopedToParticipantsAndReadAllHolders(t *testing.T) {
	repo := newFakeRepository()
	svc, _, _ := newService(repo)
	tk := createTicket(t, svc, "requester-1")

	if _, err := svc.Get(requesterContext("someone-else"), tk.ID); !apperr.Is(err, apperr.KindForbidden) {
		t.Fatalf("unrelated member: err = %v, want forbidden", err)
	}
	if _, err := svc.Get(requesterContext("requester-1"), tk.ID); err != nil {
		t.Fatalf("requester should be able to read their own ticket: %v", err)
	}
	if _, err := svc.Get(auditorContext("auditor-1"), tk.ID); err != nil {
		t.Fatalf("ticket:read_all holder should be able to read any ticket: %v", err)
	}
}

func TestListAllScopeRequiresReadAll(t *testing.T) {
	repo := newFakeRepository()
	svc, _, _ := newService(repo)
	createTicket(t, svc, "requester-1")

	if _, err := svc.List(requesterContext("requester-1"), ticket.ScopeAll, ticket.ListFilter{}, 1, 20); !apperr.Is(err, apperr.KindForbidden) {
		t.Fatalf("err = %v, want forbidden", err)
	}
	page, err := svc.List(auditorContext("auditor-1"), ticket.ScopeAll, ticket.ListFilter{}, 1, 20)
	if err != nil {
		t.Fatalf("list all as read_all holder: %v", err)
	}
	if page.Total != 1 {
		t.Fatalf("total = %d, want 1", page.Total)
	}
}

func TestListMineScopeIsForcedToTheCaller(t *testing.T) {
	repo := newFakeRepository()
	svc, _, _ := newService(repo)
	createTicket(t, svc, "requester-1")
	createTicket(t, svc, "requester-2")

	page, err := svc.List(requesterContext("requester-1"), ticket.ScopeMine, ticket.ListFilter{}, 1, 20)
	if err != nil {
		t.Fatalf("list mine: %v", err)
	}
	if page.Total != 1 {
		t.Fatalf("total = %d, want 1 (only requester-1's own ticket)", page.Total)
	}
	for _, item := range page.Items {
		if item.RequesterID != "requester-1" {
			t.Fatalf("leaked another requester's ticket: %+v", item)
		}
	}
}

func TestGetActivityIsScopedLikeGet(t *testing.T) {
	repo := newFakeRepository()
	auditRecorder := &nopAuditRecorder{}
	notify := &memoryNotifier{}
	activity := &fakeActivityReader{records: []audit.Record{{ID: "evt-1", Action: "ticket.created"}}}
	svc := ticket.NewService(repo, rollbackTx{repo: repo}, auditRecorder, activity, notify)
	tk := createTicket(t, svc, "requester-1")

	if _, err := svc.GetActivity(requesterContext("someone-else"), tk.ID); !apperr.Is(err, apperr.KindForbidden) {
		t.Fatalf("unrelated member: err = %v, want forbidden", err)
	}
	records, err := svc.GetActivity(requesterContext("requester-1"), tk.ID)
	if err != nil {
		t.Fatalf("requester reading their own ticket's activity: %v", err)
	}
	if len(records) != 1 || records[0].ID != "evt-1" {
		t.Fatalf("records = %+v, want the one seeded record", records)
	}
}

func TestNotificationFailureRollsBackTheStatusChange(t *testing.T) {
	repo := newFakeRepository()
	audit := &nopAuditRecorder{}
	notify := &memoryNotifier{failErr: apperr.Internal(errFailedDelivery)}
	svc := ticket.NewService(repo, rollbackTx{repo: repo}, audit, &fakeActivityReader{}, notify)
	tk := createTicket(t, svc, "requester-1")

	_, err := svc.Assign(managerContext("manager-1"), tk.ID, "assignee-1", tk.Version)
	if err == nil {
		t.Fatalf("expected the notification failure to fail the whole action")
	}
	reloaded, getErr := repo.GetByID(context.Background(), tk.ID)
	if getErr != nil {
		t.Fatalf("reload: %v", getErr)
	}
	if reloaded.Status != ticket.StatusPending || reloaded.AssigneeID != nil {
		t.Fatalf("ticket was not rolled back: %+v", reloaded)
	}
}

var errFailedDelivery = errors.New("notification delivery failed")
