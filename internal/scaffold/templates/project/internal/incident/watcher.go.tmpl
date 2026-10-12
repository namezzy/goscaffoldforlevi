package incident

import (
	"context"
	"log/slog"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/notification"
)

// DeadLetterOccurrence is the slice of a dead letter row the watcher needs
// to detect and merge an incident — not the full messaging.DeadLetter,
// which also carries the payload this package has no reason to duplicate.
type DeadLetterOccurrence struct {
	ID        string
	EventID   string
	EventType string
	Consumer  string
	LastError string
	FailedAt  time.Time
}

// NewIncident is the validated input for PollRepository.Create.
type NewIncident struct {
	ID           string
	Fingerprint  string
	DeadLetterID string
	EventID      string
	EventType    string
	Consumer     string
	LastError    string
	FirstSeenAt  time.Time
}

// PollRepository is Watcher's persistence port — declared separately from
// Repository (Service's port) the way messaging.OutboxStore is declared
// separately from messaging.AdminStore: detection and merging need
// different, row-locking operations that an operator-facing read/triage API
// has no business exposing.
type PollRepository interface {
	// LockPollState reads the checkpoint FOR UPDATE. Must run inside the
	// same transaction as AdvancePollState and everything between them, so
	// only one watcher replica processes a given dead letter.
	LockPollState(ctx context.Context) (time.Time, error)
	AdvancePollState(ctx context.Context, lastFailedAt time.Time) error
	// NextUnpolled returns the oldest dead letter with failed_at after
	// since, or ok=false if there is none.
	NextUnpolled(ctx context.Context, since time.Time) (occurrence DeadLetterOccurrence, ok bool, err error)
	// LockOpenByFingerprint finds the merge target, row-locked so a
	// concurrent operator action on it (ack/resolve/replay) serializes
	// with this merge instead of racing it.
	LockOpenByFingerprint(ctx context.Context, fingerprint string) (incident Incident, ok bool, err error)
	Create(ctx context.Context, in NewIncident) (Incident, error)
	MergeOccurrence(ctx context.Context, id string, lastSeenAt time.Time, deadLetterID, eventID, lastError string) error

	ListPendingReplay(ctx context.Context, limit int32) ([]Incident, error)
	// ExistsProcessedSince is the real "did the replay succeed" signal —
	// see queries/incident.sql's ExistsProcessedEventSince.
	ExistsProcessedSince(ctx context.Context, eventID, consumer string, since time.Time) (bool, error)
	ResolveReplay(ctx context.Context, id string, result ReplayStatus) (Incident, error)
}

// NotificationRecorder creates an in-app notification — matching
// ticket.NotificationRecorder's relationship to package notification.
type NotificationRecorder interface {
	Notify(ctx context.Context, n notification.NewNotification) error
}

// RecipientLookup resolves which users should be alerted about a new
// incident — declared here rather than depending on user.Service, whose
// listing enforces user:list against a caller principal the background
// watcher does not have.
type RecipientLookup interface {
	ActiveUserIDsForRoles(ctx context.Context, roles []string) ([]string, error)
}

// EventPublisher publishes to the outbox — reused for the exception
// center's external channel, the same relationship auth.EventPublisher has
// to messaging.
type EventPublisher interface {
	Publish(ctx context.Context, event messaging.Event) error
}

// EventIncidentOpened fires once per NEWLY opened incident — never on a
// merge — so a recurring failure pages someone once, not on every
// occurrence. cmd/worker's notify.IncidentAlertHandler is the external
// (email) channel that subscribes to it; see docs/security.md for why a
// fuller alerting platform (paging, escalation, SMS) is an extension point
// rather than built this phase.
const EventIncidentOpened = "incident.opened.v1"

// IncidentOpenedPayload is EventIncidentOpened's payload.
type IncidentOpenedPayload struct {
	IncidentID string `json:"incident_id"`
	EventType  string `json:"event_type"`
	Consumer   string `json:"consumer"`
	Severity   string `json:"severity"`
	LastError  string `json:"last_error"`
}

// NotifyRoles is who gets the in-app "new incident" notification: the
// roles that hold incident:manage. Fixed here rather than made a config
// option — see docs/security.md's rolePermissions-mirrors-here note.
var NotifyRoles = []string{"system_admin", "operator"}

// WatcherConfig tunes the background poller.
type WatcherConfig struct {
	// PollInterval is how often the dead-letter watcher checks for new
	// failures.
	PollInterval time.Duration
	// ReplayResolveInterval is how often pending replays are checked for a
	// real result.
	ReplayResolveInterval time.Duration
	// ReplayResolveLimit bounds one resolve sweep.
	ReplayResolveLimit int32
}

// DefaultWatcherConfig returns reasonable defaults: dead letters are
// triaged within seconds of landing, and a pending replay is re-checked
// often enough to catch success without hammering the database.
func DefaultWatcherConfig() WatcherConfig {
	return WatcherConfig{
		PollInterval:          5 * time.Second,
		ReplayResolveInterval: 10 * time.Second,
		ReplayResolveLimit:    50,
	}
}

// Watcher is the exception center's background half: it turns new dead
// letters into incidents (merging repeats by fingerprint) and learns the
// real outcome of replays that Service.RequestReplay submitted. It runs in
// cmd/worker, independent of any browser tab — detection and notification
// continue whether or not anyone has the exception center open.
type Watcher struct {
	repo       PollRepository
	tx         TxManager
	audit      AuditRecorder
	notify     NotificationRecorder
	recipients RecipientLookup
	events     EventPublisher
	cfg        WatcherConfig
	logger     *slog.Logger
}

// NewWatcher builds a watcher.
func NewWatcher(
	repo PollRepository, tx TxManager, auditRecorder AuditRecorder,
	notify NotificationRecorder, recipients RecipientLookup, events EventPublisher,
	cfg WatcherConfig, logger *slog.Logger,
) *Watcher {
	return &Watcher{
		repo: repo, tx: tx, audit: auditRecorder, notify: notify,
		recipients: recipients, events: events, cfg: cfg, logger: logger,
	}
}

// Run detects new dead letters and resolves pending replays until ctx is
// cancelled.
func (w *Watcher) Run(ctx context.Context) error {
	w.logger.InfoContext(ctx, "incident watcher started",
		slog.Duration("poll_interval", w.cfg.PollInterval),
		slog.Duration("replay_resolve_interval", w.cfg.ReplayResolveInterval))

	pollTicker := time.NewTicker(w.cfg.PollInterval)
	defer pollTicker.Stop()
	replayTicker := time.NewTicker(w.cfg.ReplayResolveInterval)
	defer replayTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.InfoContext(context.WithoutCancel(ctx), "incident watcher stopped")
			return nil
		case <-pollTicker.C:
			w.drainDeadLetters(ctx)
		case <-replayTicker.C:
			w.resolvePendingReplays(ctx)
		}
	}
}

// drainDeadLetters processes unpolled dead letters one at a time until none
// remain, mirroring messaging.Relay.drain: a burst larger than one tick
// must not trickle out at one dead letter per PollInterval.
func (w *Watcher) drainDeadLetters(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		processed, err := w.pollOnce(ctx)
		if err != nil {
			w.logger.ErrorContext(ctx, "incident poll failed", slog.Any("error", err))
			return
		}
		if !processed {
			return
		}
	}
}

// pollOnce claims the next unprocessed dead letter and either merges it into
// an existing open incident or opens a new one, all inside one transaction
// with the poll checkpoint advance — a crash between them would otherwise
// either re-process the same dead letter or silently skip it.
func (w *Watcher) pollOnce(ctx context.Context) (processed bool, err error) {
	var opened *Incident

	err = w.tx.WithTx(ctx, func(ctx context.Context) error {
		watermark, err := w.repo.LockPollState(ctx)
		if err != nil {
			return err
		}
		occ, ok, err := w.repo.NextUnpolled(ctx, watermark)
		if err != nil {
			return err
		}
		if !ok {
			processed = false
			return nil
		}
		processed = true

		fingerprint := Fingerprint(occ.Consumer, occ.EventType)
		existing, found, err := w.repo.LockOpenByFingerprint(ctx, fingerprint)
		if err != nil {
			return err
		}
		if found {
			if err := w.repo.MergeOccurrence(ctx, existing.ID, occ.FailedAt, occ.ID, occ.EventID, occ.LastError); err != nil {
				return err
			}
			if err := w.audit.Record(ctx, auditSystemEntry("incident.merged", existing.ID, map[string]any{
				"dead_letter_id": occ.ID, "occurrence_count": existing.OccurrenceCount + 1,
			})); err != nil {
				return err
			}
		} else {
			created, err := w.repo.Create(ctx, NewIncident{
				ID: NewID(), Fingerprint: fingerprint, DeadLetterID: occ.ID, EventID: occ.EventID,
				EventType: occ.EventType, Consumer: occ.Consumer, LastError: occ.LastError,
				FirstSeenAt: occ.FailedAt,
			})
			if err != nil {
				return err
			}
			if err := w.audit.Record(ctx, auditSystemEntry("incident.opened", created.ID, map[string]any{
				"event_type": created.EventType, "consumer": created.Consumer,
			})); err != nil {
				return err
			}
			opened = &created
		}

		return w.repo.AdvancePollState(ctx, occ.FailedAt)
	})
	if err != nil {
		return false, err
	}

	// Notifications happen after the transaction commits: a failure to
	// notify must not roll back an incident that was, in fact, opened.
	if opened != nil {
		w.notifyOpened(ctx, *opened)
	}
	return processed, nil
}

// auditSystemEntry builds an audit.Entry with no actor — see audit.Entry's
// doc comment on ActorID: nil is exactly right for a system-detected event,
// never a signed-in principal's action.
func auditSystemEntry(action, incidentID string, after any) audit.Entry {
	return audit.Entry{
		Action: action, TargetType: "incident", TargetID: incidentID,
		Result: audit.ResultSuccess, After: after,
	}
}

// notifyOpened tells every incident:manage holder in-app, and publishes the
// external-channel event. Both are best-effort: a notification failure must
// not be mistaken for the incident itself failing to open, and is logged
// rather than retried indefinitely, matching how ticket's post-commit
// notifications are treated.
func (w *Watcher) notifyOpened(ctx context.Context, opened Incident) {
	recipients, err := w.recipients.ActiveUserIDsForRoles(ctx, NotifyRoles)
	if err != nil {
		w.logger.WarnContext(ctx, "could not resolve incident alert recipients", slog.Any("error", err))
	}
	for _, userID := range recipients {
		err := w.notify.Notify(ctx, notification.NewNotification{
			UserID: userID, Type: "incident_opened", Title: "New exception detected",
			Body:       opened.EventType + " (" + opened.Consumer + ")",
			TargetType: "incident", TargetID: opened.ID,
			DedupKey: "incident:" + opened.ID + ":opened",
		})
		if err != nil {
			w.logger.WarnContext(ctx, "could not create incident notification",
				slog.String("user_id", userID), slog.Any("error", err))
		}
	}

	event, err := messaging.NewEvent(EventIncidentOpened, IncidentOpenedPayload{
		IncidentID: opened.ID, EventType: opened.EventType, Consumer: opened.Consumer,
		Severity: string(opened.Severity), LastError: opened.LastError,
	})
	if err != nil {
		w.logger.WarnContext(ctx, "could not build incident opened event", slog.Any("error", err))
		return
	}
	event.TraceParent = messaging.TraceParentFromContext(ctx)
	if err := w.events.Publish(ctx, event); err != nil {
		w.logger.WarnContext(ctx, "could not publish incident opened event", slog.Any("error", err))
	}
}

// resolvePendingReplays checks every incident whose last requested replay is
// still "pending" against the real outcome — see ResolveOne.
func (w *Watcher) resolvePendingReplays(ctx context.Context) {
	pending, err := w.repo.ListPendingReplay(ctx, w.cfg.ReplayResolveLimit)
	if err != nil {
		w.logger.ErrorContext(ctx, "listing pending incident replays failed", slog.Any("error", err))
		return
	}
	for _, inc := range pending {
		if err := w.resolveOne(ctx, inc); err != nil {
			w.logger.ErrorContext(ctx, "resolving incident replay failed",
				slog.String("incident_id", inc.ID), slog.Any("error", err))
		}
	}
}

// resolveOne decides one incident's pending replay outcome:
//
//   - A new occurrence landed after the replay was requested (last_seen_at
//     advanced) — the retry dead-lettered again, so the replay failed. This
//     needs no direct check against the dead letter table: pollOnce already
//     merged that occurrence onto this same incident, bumping last_seen_at,
//     by the time this runs.
//   - Otherwise, the dedupe log now has a row for the replayed event and
//     consumer stamped after the request — the handler ran to completion,
//     so the replay succeeded (see ExistsProcessedSince's doc comment for
//     why that row is trustworthy).
//   - Otherwise it is still genuinely pending and is left alone; the next
//     sweep checks again.
func (w *Watcher) resolveOne(ctx context.Context, inc Incident) error {
	if inc.ReplayRequestedAt == nil {
		return nil
	}
	if inc.LastSeenAt.After(*inc.ReplayRequestedAt) {
		return w.finishReplay(ctx, inc, ReplayFailed)
	}
	succeeded, err := w.repo.ExistsProcessedSince(ctx, inc.EventID, inc.Consumer, *inc.ReplayRequestedAt)
	if err != nil {
		return err
	}
	if !succeeded {
		return nil
	}
	return w.finishReplay(ctx, inc, ReplaySucceeded)
}

func (w *Watcher) finishReplay(ctx context.Context, inc Incident, result ReplayStatus) error {
	return w.tx.WithTx(ctx, func(ctx context.Context) error {
		if _, err := w.repo.ResolveReplay(ctx, inc.ID, result); err != nil {
			return err
		}
		return w.audit.Record(ctx, auditSystemEntry("incident.replay_resolved", inc.ID,
			map[string]any{"result": result}))
	})
}
