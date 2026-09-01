package messaging

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// OutboxStore is the relay's view of the outbox. It is declared here, next to
// its only consumer, rather than exported from the storage package: the relay
// states what it needs and the database package happens to satisfy it.
type OutboxStore interface {
	// ClaimBatch locks a batch of undelivered events and passes them to fn.
	// The transaction is the store's business: the relay needs the batch
	// delivered under a lock, not a transaction manager of its own.
	ClaimBatch(
		ctx context.Context,
		limit int32,
		fn func(ctx context.Context, events []PendingOutboxEvent) error,
	) error
	MarkPublished(ctx context.Context, id uuid.UUID) error
	RecordFailure(ctx context.Context, id uuid.UUID, cause error) error
	PendingCount(ctx context.Context) (int64, error)
	PrunePublished(ctx context.Context, cutoff time.Time) (int64, error)
	PruneProcessed(ctx context.Context, cutoff time.Time) (int64, error)
}

// PendingOutboxEvent is an event awaiting delivery.
type PendingOutboxEvent struct {
	Event    Event
	Attempts int32
}

// RelayConfig tunes the outbox relay.
type RelayConfig struct {
	// PollInterval is how long the relay waits after finding nothing. It is
	// also the floor on end-to-end latency, which is the price of not needing
	// LISTEN/NOTIFY (ADR-0009).
	PollInterval time.Duration
	// BatchSize bounds one claim. It bounds the lock duration too, since the
	// batch is published while its rows are locked.
	BatchSize int32
	// PublishTimeout bounds one delivery.
	PublishTimeout time.Duration
	// PublishedRetention and ProcessedRetention are how long delivered events
	// and dedupe records are kept.
	PublishedRetention time.Duration
	ProcessedRetention time.Duration
	// PruneInterval is how often the retention sweep runs.
	PruneInterval time.Duration
}

// DefaultRelayConfig returns the settings ADR-0009 specifies.
func DefaultRelayConfig() RelayConfig {
	return RelayConfig{
		PollInterval:       time.Second,
		BatchSize:          100,
		PublishTimeout:     10 * time.Second,
		PublishedRetention: 7 * 24 * time.Hour,
		ProcessedRetention: 30 * 24 * time.Hour,
		PruneInterval:      time.Hour,
	}
}

// Relay moves committed events from the outbox to the broker.
//
// It is a separate process from the request that produced the event, and that
// separation is the point: the request is finished and its result is durable
// before the relay ever runs, so a broker outage delays events instead of
// failing writes.
type Relay struct {
	store     OutboxStore
	publisher Publisher
	cfg       RelayConfig
	logger    *slog.Logger

	// wake lets a producer tell the relay not to wait out its poll interval.
	// It is buffered with capacity one because the signal is "there is work",
	// which does not accumulate.
	wake chan struct{}

	// observer is never nil; see SetObserver.
	observer Observer
}

// NewRelay builds a relay.
func NewRelay(
	store OutboxStore,
	publisher Publisher,
	cfg RelayConfig,
	logger *slog.Logger,
) *Relay {
	return &Relay{
		observer:  NopObserver(),
		store:     store,
		publisher: publisher,
		cfg:       cfg,
		logger:    logger,
		wake:      make(chan struct{}, 1),
	}
}

// Notify hints that new events exist, so the relay does not sleep out its
// interval. Dropping the hint when one is already pending is correct: the
// relay drains everything it finds, so a second hint would add nothing.
func (r *Relay) Notify() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// Run delivers events until ctx is cancelled.
//
// Cancellation is a clean stop, not an abort: the batch in flight finishes
// first. Anything still unpublished stays in the table, which is why shutting
// down cannot lose events — the relay owns no state that a restart would not
// rediscover.
func (r *Relay) Run(ctx context.Context) error {
	r.logger.InfoContext(ctx, "outbox relay started",
		slog.Duration("poll_interval", r.cfg.PollInterval),
		slog.Int("batch_size", int(r.cfg.BatchSize)))

	ticker := time.NewTicker(r.cfg.PollInterval)
	defer ticker.Stop()
	pruneTicker := time.NewTicker(r.cfg.PruneInterval)
	defer pruneTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.InfoContext(context.WithoutCancel(ctx), "outbox relay stopped")
			return nil
		case <-pruneTicker.C:
			r.prune(ctx)
		case <-ticker.C:
			r.drain(ctx)
		case <-r.wake:
			r.drain(ctx)
		}
	}
}

// drain publishes batches until one comes back short, which means the table is
// empty. Without this loop a burst larger than one batch would trickle out at
// BatchSize per poll interval.
func (r *Relay) drain(ctx context.Context) {
	// Reported at the end of the sweep, not from a ticker of its own. A
	// separate ticker is a second thing that can stop, and a gauge that keeps
	// updating while the relay is wedged would keep reporting the last depth
	// it managed to read — which is the one number that must not be stale.
	defer r.reportPending(ctx)

	for {
		if ctx.Err() != nil {
			return
		}
		delivered, err := r.deliverBatch(ctx)
		if err != nil {
			r.logger.ErrorContext(ctx, "outbox batch failed", slog.Any("error", err))
			return
		}
		if delivered < int(r.cfg.BatchSize) {
			return
		}
	}
}

// reportPending publishes the outbox depth to the observer.
//
// A failure to read it is logged and dropped: the depth is diagnostic, and
// failing the sweep over it would let a broken gauge stop delivery.
func (r *Relay) reportPending(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	pending, err := r.store.PendingCount(ctx)
	if err != nil {
		r.logger.WarnContext(ctx, "could not read outbox depth", slog.Any("error", err))
		return
	}
	r.observer.MessagingOutboxPending(pending)
}

// deliverBatch publishes one claimed batch and reports how many rows it saw.
func (r *Relay) deliverBatch(ctx context.Context) (int, error) {
	seen := 0

	err := r.store.ClaimBatch(ctx, r.cfg.BatchSize,
		func(ctx context.Context, events []PendingOutboxEvent) error {
			seen = len(events)

			for _, pending := range events {
				// One event failing must not abandon the rest of the batch:
				// a single poisonous message would otherwise block every event
				// behind it for as long as it kept failing.
				if err := r.deliverOne(ctx, pending); err != nil {
					r.logger.ErrorContext(ctx, "event delivery failed",
						slog.String("event_id", pending.Event.ID.String()),
						slog.String("event_type", pending.Event.Type),
						slog.Int("attempts", int(pending.Attempts)+1),
						slog.Any("error", err))

					if recordErr := r.store.RecordFailure(ctx, pending.Event.ID, err); recordErr != nil {
						// The failure counter is diagnostic; losing it must not
						// roll back the rows that did publish.
						r.logger.ErrorContext(ctx, "could not record delivery failure",
							slog.Any("error", recordErr))
					}
					continue
				}

				// Counted here rather than at the outbox write: the metric
				// answers "is anything reaching the broker", and a relay that
				// had stopped delivering must not still look busy.
				r.observer.MessagingPublished(pending.Event.Type)

				if err := r.store.MarkPublished(ctx, pending.Event.ID); err != nil {
					// The event is already out. Failing here rolls back the
					// mark, the next sweep publishes it again, and consumers
					// deduplicate — the one duplicate is the accepted cost of
					// at-least-once.
					return err
				}
			}
			return nil
		})
	if err != nil {
		return seen, err
	}
	return seen, nil
}

func (r *Relay) deliverOne(ctx context.Context, pending PendingOutboxEvent) error {
	// The publish gets its own deadline. Inheriting the batch context would
	// let one unresponsive broker call hold the row locks for as long as it
	// liked, and those locks are what other relays are waiting on.
	publishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.cfg.PublishTimeout)
	defer cancel()

	return r.publisher.Publish(publishCtx, pending.Event)
}

// prune drops delivered events and expired dedupe records.
func (r *Relay) prune(ctx context.Context) {
	now := time.Now()

	published, err := r.store.PrunePublished(ctx, now.Add(-r.cfg.PublishedRetention))
	if err != nil {
		r.logger.ErrorContext(ctx, "pruning published events failed", slog.Any("error", err))
	}

	processed, err := r.store.PruneProcessed(ctx, now.Add(-r.cfg.ProcessedRetention))
	if err != nil {
		r.logger.ErrorContext(ctx, "pruning processed events failed", slog.Any("error", err))
	}

	if published > 0 || processed > 0 {
		r.logger.InfoContext(ctx, "messaging retention sweep",
			slog.Int64("published_events_deleted", published),
			slog.Int64("processed_events_deleted", processed))
	}
}

// ErrRelayStopped is returned by helpers that need a sentinel for a relay that
// is no longer running.
var ErrRelayStopped = errors.New("messaging: relay stopped")

// SetObserver attaches instrumentation. Passing nil restores the no-op, so a
// caller that has not configured metrics cannot leave a nil interface on the
// delivery path.
func (r *Relay) SetObserver(o Observer) {
	if o == nil {
		o = NopObserver()
	}
	r.observer = o
}
