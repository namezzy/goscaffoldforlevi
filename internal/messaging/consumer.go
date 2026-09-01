package messaging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
)

// DedupeStore records which events a consumer has already handled.
//
// ClaimEvent must run inside the same transaction as the side effect. The
// interface cannot enforce that, so the implementation checks it: split across
// two transactions the guarantee quietly disappears and the only symptom is a
// duplicated side effect, long after the code that caused it was written.
type DedupeStore interface {
	// WithTx runs fn in a transaction. The consumer owns the transaction
	// boundary because the boundary is the guarantee.
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
	// ClaimEvent reports whether the event still needs processing.
	ClaimEvent(ctx context.Context, eventID uuid.UUID, consumer string) (bool, error)
	// RecordDeadLetter parks an event that will never succeed.
	RecordDeadLetter(
		ctx context.Context, event Event, consumer string, attempts int, cause error,
	) error
}

// ConsumerConfig tunes one consumer.
type ConsumerConfig struct {
	// Name identifies the consumer in the dedupe log and in dead letters. Two
	// consumers of the same event must not share a name, or one would suppress
	// the other's work.
	Name string
	// HandlerTimeout bounds one attempt.
	HandlerTimeout time.Duration
	// MaxAttempts is the number of tries before the event is parked.
	MaxAttempts int
	// BaseBackoff is the first retry delay; each attempt doubles the ceiling.
	BaseBackoff time.Duration
	// MaxBackoff caps the ceiling, so a long-lived outage does not push the
	// next attempt hours away.
	MaxBackoff time.Duration
}

// DefaultConsumerConfig returns the retry schedule from ADR-0009: five
// attempts, backing off 1s, 2s, 4s, 8s, then dead-lettering.
func DefaultConsumerConfig(name string) ConsumerConfig {
	return ConsumerConfig{
		Name:           name,
		HandlerTimeout: 30 * time.Second,
		MaxAttempts:    5,
		BaseBackoff:    time.Second,
		MaxBackoff:     30 * time.Second,
	}
}

// ErrPermanent marks a failure that retrying cannot fix.
//
// A malformed payload is the ordinary case: it will be exactly as malformed on
// the fifth attempt, so the four extra tries buy nothing and delay the moment
// a human sees the problem.
var ErrPermanent = errors.New("messaging: permanent failure")

// Permanent wraps err so the consumer parks the event immediately.
func Permanent(err error) error { return fmt.Errorf("%w: %w", ErrPermanent, err) }

// Consumer runs a handler for one event at a time, with deduplication,
// bounded retries and a dead letter of last resort.
//
// It is not a subscription. Where the events come from — NATS, a test slice,
// a replay — is the caller's business; this type owns what happens to one
// event once it arrives.
type Consumer struct {
	handler Handler
	store   DedupeStore
	cfg     ConsumerConfig
	logger  *slog.Logger

	// sleep is injectable so retry-schedule tests do not spend real seconds
	// waiting. The schedule is the thing under test, not the clock.
	sleep func(ctx context.Context, d time.Duration) error
}

// NewConsumer builds a consumer.
func NewConsumer(
	handler Handler, store DedupeStore, cfg ConsumerConfig, logger *slog.Logger,
) *Consumer {
	return &Consumer{
		handler: handler,
		store:   store,
		cfg:     cfg,
		logger:  logger,
		sleep:   sleepCtx,
	}
}

// Process handles one event to completion: it returns nil once the event has
// been handled, recognised as already handled, or parked as a dead letter.
//
// A non-nil return means the event was neither handled nor parked, so the
// caller must not acknowledge it. That is the only case where redelivery is
// still needed, and keeping it narrow is what stops a broken database from
// looking like a broken message.
func (c *Consumer) Process(ctx context.Context, event Event) error {
	var lastErr error

	for attempt := 1; attempt <= c.cfg.MaxAttempts; attempt++ {
		handled, err := c.attempt(ctx, event, attempt)
		if err == nil {
			if handled {
				c.logger.DebugContext(ctx, "event handled",
					slog.String("consumer", c.cfg.Name),
					slog.String("event_id", event.ID.String()),
					slog.String("event_type", event.Type),
					slog.Int("attempt", attempt))
			}
			return nil
		}
		lastErr = err

		// Cancellation is a shutdown, not a failure of the event. Parking it
		// would turn every deploy into a pile of dead letters.
		if ctx.Err() != nil {
			return fmt.Errorf("consumer %s stopped while processing %s: %w",
				c.cfg.Name, event.ID, err)
		}

		if errors.Is(err, ErrPermanent) {
			c.logger.WarnContext(ctx, "event failed permanently",
				slog.String("consumer", c.cfg.Name),
				slog.String("event_id", event.ID.String()),
				slog.String("event_type", event.Type),
				slog.Any("error", err))
			return c.park(ctx, event, attempt, err)
		}

		if attempt == c.cfg.MaxAttempts {
			break
		}

		delay := c.backoff(attempt)
		c.logger.WarnContext(ctx, "event failed, retrying",
			slog.String("consumer", c.cfg.Name),
			slog.String("event_id", event.ID.String()),
			slog.String("event_type", event.Type),
			slog.Int("attempt", attempt),
			slog.Duration("retry_in", delay),
			slog.Any("error", err))

		if err := c.sleep(ctx, delay); err != nil {
			return fmt.Errorf("consumer %s stopped while backing off for %s: %w",
				c.cfg.Name, event.ID, lastErr)
		}
	}

	c.logger.ErrorContext(ctx, "event exhausted its retries",
		slog.String("consumer", c.cfg.Name),
		slog.String("event_id", event.ID.String()),
		slog.String("event_type", event.Type),
		slog.Int("attempts", c.cfg.MaxAttempts),
		slog.Any("error", lastErr))

	return c.park(ctx, event, c.cfg.MaxAttempts, lastErr)
}

// attempt runs the handler once, inside the transaction that also claims the
// event. It reports whether the handler actually ran.
func (c *Consumer) attempt(ctx context.Context, event Event, attempt int) (bool, error) {
	handled := false

	err := c.store.WithTx(ctx, func(ctx context.Context) error {
		fresh, err := c.store.ClaimEvent(ctx, event.ID, c.cfg.Name)
		if err != nil {
			return fmt.Errorf("claim event: %w", err)
		}
		if !fresh {
			// Already processed. This is the normal outcome of a redelivery,
			// not an error, and it is why at-least-once is safe here.
			c.logger.DebugContext(ctx, "event already processed, skipping",
				slog.String("consumer", c.cfg.Name),
				slog.String("event_id", event.ID.String()))
			return nil
		}

		// The handler shares the transaction with the claim. Either both land
		// or neither does, so a crash cannot leave the event marked done with
		// its work undone.
		handlerCtx, cancel := context.WithTimeout(ctx, c.cfg.HandlerTimeout)
		defer cancel()

		if err := c.runHandler(handlerCtx, event); err != nil {
			return err
		}
		handled = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("attempt %d: %w", attempt, err)
	}
	return handled, nil
}

// runHandler calls the handler and converts a panic into an error.
//
// A panicking handler must not kill the consumer: one malformed message would
// otherwise stop every other message in the system, which is a far worse
// outcome than the bad message being retried and parked.
func (c *Consumer) runHandler(ctx context.Context, event Event) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			// Parked rather than retried: a panic is a bug, and the same bug
			// will panic again on every attempt.
			err = Permanent(fmt.Errorf("handler panicked: %v", recovered))
			c.logger.ErrorContext(ctx, "handler panicked",
				slog.String("consumer", c.cfg.Name),
				slog.String("event_id", event.ID.String()),
				slog.String("event_type", event.Type),
				slog.Any("panic", recovered))
		}
	}()

	return c.handler.Handle(ctx, event)
}

// park records a dead letter.
func (c *Consumer) park(ctx context.Context, event Event, attempts int, cause error) error {
	// A shutdown must still be able to park: without WithoutCancel the event
	// would be neither handled nor recorded, which is the one state the
	// system has no way to notice.
	parkCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	if err := c.store.RecordDeadLetter(parkCtx, event, c.cfg.Name, attempts, cause); err != nil {
		return fmt.Errorf("record dead letter for %s: %w", event.ID, err)
	}
	return nil
}

// backoff returns the delay before the next attempt, with full jitter.
//
// The delay is uniform over [0, ceiling) rather than ceiling itself. That
// matters when a shared dependency fails: every consumer that queued up during
// the outage would otherwise retry in the same instant and knock it back over.
func (c *Consumer) backoff(attempt int) time.Duration {
	ceiling := c.cfg.BaseBackoff << (attempt - 1)
	if ceiling > c.cfg.MaxBackoff || ceiling <= 0 {
		ceiling = c.cfg.MaxBackoff
	}
	if ceiling <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(ceiling))) //nolint:gosec // jitter, not a secret
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
