package incident_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/incident"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/notification"
)

// fakePollStore is an in-memory stand-in for incident.PollRepository, kept
// separate from fakeRepository (incident_test.go) the same way incidentpg's
// real Store implements both ports but the two are declared as separate
// interfaces for separate consumers.
type fakePollStore struct {
	byID        map[string]incident.Incident
	watermark   time.Time
	deadLetters []incident.DeadLetterOccurrence
	processed   map[string]time.Time // "eventID:consumer" -> processedAt
}

func newFakePollStore() *fakePollStore {
	return &fakePollStore{byID: map[string]incident.Incident{}, processed: map[string]time.Time{}}
}

func (f *fakePollStore) LockPollState(context.Context) (time.Time, error) { return f.watermark, nil }

func (f *fakePollStore) AdvancePollState(_ context.Context, t time.Time) error {
	f.watermark = t
	return nil
}

func (f *fakePollStore) NextUnpolled(_ context.Context, since time.Time) (incident.DeadLetterOccurrence, bool, error) {
	sorted := append([]incident.DeadLetterOccurrence(nil), f.deadLetters...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].FailedAt.Before(sorted[j].FailedAt) })
	for _, occ := range sorted {
		if occ.FailedAt.After(since) {
			return occ, true, nil
		}
	}
	return incident.DeadLetterOccurrence{}, false, nil
}

func (f *fakePollStore) LockOpenByFingerprint(_ context.Context, fingerprint string) (incident.Incident, bool, error) {
	for _, inc := range f.byID {
		if inc.Fingerprint == fingerprint && inc.Status != incident.StatusResolved {
			return inc, true, nil
		}
	}
	return incident.Incident{}, false, nil
}

func (f *fakePollStore) Create(_ context.Context, in incident.NewIncident) (incident.Incident, error) {
	inc := incident.Incident{
		ID: in.ID, Fingerprint: in.Fingerprint, Severity: incident.SeverityWarning, Status: incident.StatusOpen,
		DeadLetterID: in.DeadLetterID, EventID: in.EventID, EventType: in.EventType, Consumer: in.Consumer,
		LastError: in.LastError, OccurrenceCount: 1, FirstSeenAt: in.FirstSeenAt, LastSeenAt: in.FirstSeenAt,
		CreatedAt: in.FirstSeenAt, UpdatedAt: in.FirstSeenAt,
	}
	f.byID[inc.ID] = inc
	return inc, nil
}

func (f *fakePollStore) MergeOccurrence(_ context.Context, id string, lastSeenAt time.Time, deadLetterID, eventID, lastError string) error {
	inc := f.byID[id]
	inc.OccurrenceCount++
	inc.LastSeenAt = lastSeenAt
	inc.DeadLetterID = deadLetterID
	inc.EventID = eventID
	inc.LastError = lastError
	f.byID[id] = inc
	return nil
}

func (f *fakePollStore) ListPendingReplay(context.Context, int32) ([]incident.Incident, error) {
	var out []incident.Incident
	for _, inc := range f.byID {
		if inc.ReplayStatus != nil && *inc.ReplayStatus == incident.ReplayPending {
			out = append(out, inc)
		}
	}
	return out, nil
}

func (f *fakePollStore) ExistsProcessedSince(_ context.Context, eventID, consumer string, since time.Time) (bool, error) {
	at, ok := f.processed[eventID+":"+consumer]
	return ok && at.After(since), nil
}

func (f *fakePollStore) ResolveReplay(_ context.Context, id string, result incident.ReplayStatus) (incident.Incident, error) {
	inc := f.byID[id]
	inc.ReplayStatus = &result
	now := time.Now()
	inc.ReplayResultAt = &now
	f.byID[id] = inc
	return inc, nil
}

type fakeNotificationRecorder struct {
	sent []notification.NewNotification
}

func (f *fakeNotificationRecorder) Notify(_ context.Context, n notification.NewNotification) error {
	f.sent = append(f.sent, n)
	return nil
}

type fakeRecipientLookup struct{ ids []string }

func (f *fakeRecipientLookup) ActiveUserIDsForRoles(context.Context, []string) ([]string, error) {
	return f.ids, nil
}

type fakeEventPublisher struct{ events []messaging.Event }

func (f *fakeEventPublisher) Publish(_ context.Context, e messaging.Event) error {
	f.events = append(f.events, e)
	return nil
}

type pollFakeTx struct{ store *fakePollStore }

func (tx pollFakeTx) WithTx(ctx context.Context, fn func(context.Context) error) error {
	// No rollback semantics needed here: every watcher test asserts on
	// success paths only, matching messaging.Relay's own test style — the
	// interesting behaviour under test is the merge rule and the replay
	// resolution logic, not transactional atomicity (that is what
	// internal/incident/incidentpg's real-Postgres integration test covers).
	return fn(ctx)
}

func newWatcher(store *fakePollStore) (*incident.Watcher, *fakeNotificationRecorder, *fakeEventPublisher) {
	notify := &fakeNotificationRecorder{}
	events := &fakeEventPublisher{}
	recipients := &fakeRecipientLookup{ids: []string{"admin-1", "operator-1"}}
	w := incident.NewWatcher(
		store, pollFakeTx{store: store}, &nopAuditRecorder{}, notify, recipients, events,
		incident.DefaultWatcherConfig(), incident.DiscardLoggerForTest(),
	)
	return w, notify, events
}

func occurrence(consumer, eventType, lastError string, failedAt time.Time) incident.DeadLetterOccurrence {
	return incident.DeadLetterOccurrence{
		ID: uuid.Must(uuid.NewV7()).String(), EventID: uuid.Must(uuid.NewV7()).String(),
		EventType: eventType, Consumer: consumer, LastError: lastError, FailedAt: failedAt,
	}
}

func TestPollOnceOpensANewIncidentAndNotifiesEveryRecipient(t *testing.T) {
	t.Parallel()
	store := newFakePollStore()
	occ := occurrence("worker", "user.created.v1", "boom", time.Now())
	store.deadLetters = append(store.deadLetters, occ)
	w, notify, events := newWatcher(store)

	drainOnce(t, w)

	if len(store.byID) != 1 {
		t.Fatalf("incidents created = %d, want 1", len(store.byID))
	}
	var inc incident.Incident
	for _, v := range store.byID {
		inc = v
	}
	if inc.OccurrenceCount != 1 || inc.Fingerprint != "worker:user.created.v1" {
		t.Errorf("incident = %+v, unexpected", inc)
	}
	if len(notify.sent) != 2 {
		t.Errorf("notifications sent = %d, want 2 (one per recipient)", len(notify.sent))
	}
	if len(events.events) != 1 || events.events[0].Type != incident.EventIncidentOpened {
		t.Errorf("published events = %+v, want one incident.opened.v1", events.events)
	}
}

// TestPollOnceMergesARepeatFailureIntoTheSameIncident is the merge/dedup
// rule the exception center exists to enforce: a second, distinct dead
// letter of the same consumer+event_type bumps the existing open incident
// instead of paging anyone again.
func TestPollOnceMergesARepeatFailureIntoTheSameIncident(t *testing.T) {
	t.Parallel()
	store := newFakePollStore()
	first := occurrence("worker", "user.created.v1", "boom", time.Now())
	second := occurrence("worker", "user.created.v1", "boom again", time.Now().Add(time.Second))
	store.deadLetters = append(store.deadLetters, first, second)
	w, notify, events := newWatcher(store)

	drainOnce(t, w) // opens the incident for `first`
	drainOnce(t, w) // merges `second` onto it

	if len(store.byID) != 1 {
		t.Fatalf("incidents = %d, want 1 (merged, not duplicated)", len(store.byID))
	}
	var inc incident.Incident
	for _, v := range store.byID {
		inc = v
	}
	if inc.OccurrenceCount != 2 {
		t.Errorf("occurrence_count = %d, want 2", inc.OccurrenceCount)
	}
	if inc.DeadLetterID != second.ID || inc.LastError != second.LastError {
		t.Errorf("incident did not move to the newest occurrence: %+v", inc)
	}
	// Exactly one notification round (from the first, opening poll) — the
	// merge must not page anyone a second time for the same fingerprint.
	if len(notify.sent) != 2 {
		t.Errorf("notifications sent = %d, want 2 (only from the opening occurrence)", len(notify.sent))
	}
	if len(events.events) != 1 {
		t.Errorf("published events = %d, want 1 (only from the opening occurrence)", len(events.events))
	}
}

// TestPollOnceOpensAFreshIncidentAfterTheEarlierOneWasResolved is the other
// half of the merge rule: resolving is a claim the underlying cause was
// fixed, so a later recurrence of the same fingerprint must not reopen that
// closed history.
func TestPollOnceOpensAFreshIncidentAfterTheEarlierOneWasResolved(t *testing.T) {
	t.Parallel()
	store := newFakePollStore()
	first := occurrence("worker", "user.created.v1", "boom", time.Now())
	store.deadLetters = append(store.deadLetters, first)
	w, _, _ := newWatcher(store)
	drainOnce(t, w)

	var firstID string
	for id, inc := range store.byID {
		firstID = id
		inc.Status = incident.StatusResolved
		store.byID[id] = inc
	}

	second := occurrence("worker", "user.created.v1", "boom", time.Now().Add(time.Minute))
	store.deadLetters = append(store.deadLetters, second)
	drainOnce(t, w)

	if len(store.byID) != 2 {
		t.Fatalf("incidents = %d, want 2 (a fresh one after resolution)", len(store.byID))
	}
	if store.byID[firstID].OccurrenceCount != 1 {
		t.Errorf("the resolved incident's occurrence_count changed: %+v", store.byID[firstID])
	}
}

func drainOnce(t *testing.T, w *incident.Watcher) {
	t.Helper()
	// pollOnce is unexported; exercised through the same mechanism Run
	// uses. See export_test.go for the test-only accessor.
	if err := incident.PollOnceForTest(w, context.Background()); err != nil {
		t.Fatalf("pollOnce() = %v", err)
	}
}

func TestResolvePendingReplayMarksSucceededWhenTheEventWasProcessed(t *testing.T) {
	t.Parallel()
	store := newFakePollStore()
	occ := occurrence("worker", "user.created.v1", "boom", time.Now())
	store.deadLetters = append(store.deadLetters, occ)
	w, _, _ := newWatcher(store)
	drainOnce(t, w)

	var inc incident.Incident
	for _, v := range store.byID {
		inc = v
	}
	requestedAt := time.Now()
	pending := incident.ReplayPending
	inc.ReplayStatus = &pending
	inc.ReplayRequestedAt = &requestedAt
	store.byID[inc.ID] = inc
	store.processed[inc.EventID+":"+inc.Consumer] = requestedAt.Add(time.Second)

	incident.ResolvePendingReplaysForTest(w, context.Background())

	got := store.byID[inc.ID]
	if got.ReplayStatus == nil || *got.ReplayStatus != incident.ReplaySucceeded {
		t.Fatalf("replay status = %v, want succeeded", got.ReplayStatus)
	}
}

func TestResolvePendingReplayMarksFailedWhenTheRetryDeadLetteredAgain(t *testing.T) {
	t.Parallel()
	store := newFakePollStore()
	occ := occurrence("worker", "user.created.v1", "boom", time.Now())
	store.deadLetters = append(store.deadLetters, occ)
	w, _, _ := newWatcher(store)
	drainOnce(t, w)

	var inc incident.Incident
	for _, v := range store.byID {
		inc = v
	}
	requestedAt := time.Now()
	pending := incident.ReplayPending
	inc.ReplayStatus = &pending
	inc.ReplayRequestedAt = &requestedAt
	store.byID[inc.ID] = inc

	// The retry failed and re-dead-lettered: a new occurrence of the same
	// fingerprint lands after the replay was requested.
	retryFailure := occurrence("worker", "user.created.v1", "boom", requestedAt.Add(time.Second))
	store.deadLetters = append(store.deadLetters, retryFailure)
	drainOnce(t, w)

	incident.ResolvePendingReplaysForTest(w, context.Background())

	got := store.byID[inc.ID]
	if got.ReplayStatus == nil || *got.ReplayStatus != incident.ReplayFailed {
		t.Fatalf("replay status = %v, want failed", got.ReplayStatus)
	}
}

func TestResolvePendingReplayLeavesAGenuinelyUndecidedReplayPending(t *testing.T) {
	t.Parallel()
	store := newFakePollStore()
	occ := occurrence("worker", "user.created.v1", "boom", time.Now())
	store.deadLetters = append(store.deadLetters, occ)
	w, _, _ := newWatcher(store)
	drainOnce(t, w)

	var inc incident.Incident
	for _, v := range store.byID {
		inc = v
	}
	requestedAt := time.Now()
	pending := incident.ReplayPending
	inc.ReplayStatus = &pending
	inc.ReplayRequestedAt = &requestedAt
	store.byID[inc.ID] = inc

	incident.ResolvePendingReplaysForTest(w, context.Background())

	got := store.byID[inc.ID]
	if got.ReplayStatus == nil || *got.ReplayStatus != incident.ReplayPending {
		t.Fatalf("replay status = %v, want still pending", got.ReplayStatus)
	}
}
