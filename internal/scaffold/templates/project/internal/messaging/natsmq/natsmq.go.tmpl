// Package natsmq is the NATS JetStream transport.
//
// It is the only package in the tree allowed to import nats.go, which
// .golangci.yml enforces for the business packages. Everything above it speaks
// messaging.Event, so replacing JetStream with Kafka is a change to this
// package and to the wiring in main, and to nothing else.
package natsmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"go.opentelemetry.io/otel/codes"

	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/telemetry"
)

// Config describes the connection and the stream.
type Config struct {
	// URLs are the servers to connect to.
	URLs []string
	// StreamName is the JetStream stream that stores the events.
	StreamName string
	// SubjectPrefix namespaces this service's subjects, so several services
	// can share a cluster without colliding.
	SubjectPrefix string
	// ConsumerName is the durable consumer. Durable matters: an ephemeral
	// consumer forgets its position, so a restart either replays everything or
	// skips whatever arrived while it was down.
	ConsumerName string
	// ConnectTimeout bounds the initial connection.
	ConnectTimeout time.Duration
	// PublishTimeout bounds one publish.
	PublishTimeout time.Duration
	// MaxAge is how long the stream keeps a message. It has to outlast the
	// longest outage a consumer is expected to survive.
	MaxAge time.Duration
	// AckWait is how long JetStream waits for an ack before redelivering. It
	// must exceed the consumer's total retry budget, or a message still being
	// retried gets redelivered in parallel with itself.
	AckWait time.Duration
	// MaxDeliver caps redeliveries at the broker. The consumer's own retry
	// loop and dead letter table are the real policy; this is a backstop
	// against a message being redelivered forever if the process keeps dying.
	MaxDeliver int
}

// DefaultConfig returns settings that match ADR-0009's retry budget.
func DefaultConfig() Config {
	return Config{
		URLs:           []string{nats.DefaultURL},
		StreamName:     "EVENTS",
		SubjectPrefix:  "events",
		ConsumerName:   "worker",
		ConnectTimeout: 5 * time.Second,
		PublishTimeout: 10 * time.Second,
		MaxAge:         7 * 24 * time.Hour,
		// Comfortably above five attempts with the configured backoff, so the
		// broker never redelivers a message the consumer is still working on.
		AckWait:    5 * time.Minute,
		MaxDeliver: 10,
	}
}

// headerEventID and friends carry the envelope beside the payload.
//
// The fields go in headers rather than being wrapped around the payload so
// that a consumer written in another language, or a human reading `nats sub`,
// can see what a message is without parsing it.
const (
	headerEventID     = "Event-Id"
	headerEventType   = "Event-Type"
	headerOccurredAt  = "Event-Occurred-At"
	headerTraceParent = "traceparent"
)

// Conn owns the NATS connection and the stream.
type Conn struct {
	nc     *nats.Conn
	js     jetstream.JetStream
	stream jetstream.Stream
	cfg    Config
	logger *slog.Logger
}

// Connect dials NATS and ensures the stream exists.
//
// Creating the stream here rather than in a deployment script keeps the
// service runnable from a bare broker, which is what makes the integration
// tests and a local `docker run nats` work without setup steps that only
// live in someone's shell history.
func Connect(ctx context.Context, cfg Config, logger *slog.Logger) (*Conn, error) {
	nc, err := nats.Connect(strings.Join(cfg.URLs, ","),
		nats.Timeout(cfg.ConnectTimeout),
		// Reconnect forever. A broker restart should be a pause, not an
		// outage that requires restarting every service that talked to it.
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			logger.Warn("nats disconnected", slog.Any("error", err))
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			logger.Info("nats reconnected", slog.String("url", c.ConnectedUrl()))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("open jetstream: %w", err)
	}

	stream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     cfg.StreamName,
		Subjects: []string{cfg.SubjectPrefix + ".>"},
		// The outbox is the durable record of what happened; the stream is a
		// delivery mechanism. Keeping messages by age is enough, and limits
		// rather than discarding the oldest silently.
		Retention: jetstream.LimitsPolicy,
		MaxAge:    cfg.MaxAge,
		Storage:   jetstream.FileStorage,
		// Duplicate suppression at the broker is a convenience, not the
		// guarantee: the consumer's dedupe log is what actually makes
		// redelivery safe, because it covers redelivery after the window too.
		Duplicates: 2 * time.Minute,
	})
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("create stream %s: %w", cfg.StreamName, err)
	}

	logger.InfoContext(ctx, "connected to nats",
		slog.String("url", nc.ConnectedUrl()),
		slog.String("stream", cfg.StreamName))

	return &Conn{nc: nc, js: js, stream: stream, cfg: cfg, logger: logger}, nil
}

// Close drains and closes the connection.
func (c *Conn) Close() {
	// Drain rather than Close: it flushes what is already buffered instead of
	// dropping it, which is the difference between a clean shutdown and a
	// handful of lost publishes.
	if err := c.nc.Drain(); err != nil {
		c.logger.Warn("draining nats connection failed", slog.Any("error", err))
		c.nc.Close()
	}
}

// Publisher publishes events to JetStream.
type Publisher struct {
	conn *Conn

	// tracer is never nil; see SetTracer.
	tracer telemetry.TracerProvider
}

// NewPublisher returns a publisher on conn.
func NewPublisher(conn *Conn) *Publisher {
	return &Publisher{conn: conn, tracer: telemetry.NoopTracerProvider()}
}

// SetTracer attaches tracing. Passing nil restores the no-op provider, so a
// caller that has not configured tracing cannot leave a nil interface on the
// publish path.
func (p *Publisher) SetTracer(tp telemetry.TracerProvider) {
	if tp == nil {
		tp = telemetry.NoopTracerProvider()
	}
	p.tracer = tp
}

// Publish sends one event and waits for the broker to acknowledge storage.
//
// The wait is not optional. A fire-and-forget publish returns success as soon
// as the bytes leave the process, so a broker that is up but unable to persist
// looks exactly like a healthy one, and the relay marks the event delivered.
func (p *Publisher) Publish(ctx context.Context, event messaging.Event) error {
	ctx, span := startPublishSpan(ctx, p.tracer, event)
	defer span.End()

	msg := &nats.Msg{
		Subject: subjectFor(p.conn.cfg.SubjectPrefix, event.Type),
		Data:    event.Payload,
		Header: nats.Header{
			headerEventID:    []string{event.ID.String()},
			headerEventType:  []string{event.Type},
			headerOccurredAt: []string{event.OccurredAt.Format(time.RFC3339Nano)},
		},
	}
	if header := traceParentForMessage(ctx, event.TraceParent); header != "" {
		msg.Header.Set(headerTraceParent, header)
	}

	ctx, cancel := context.WithTimeout(ctx, p.conn.cfg.PublishTimeout)
	defer cancel()

	// The event ID is the deduplication key, so a relay that publishes twice
	// after crashing before the mark produces one stored message. A replay
	// keeps the ID, so its generation is part of the key: otherwise a replay
	// inside the stream's duplicate window is silently dropped.
	_, err := p.conn.js.PublishMsg(ctx, msg, jetstream.WithMsgID(msgIDFor(event)))
	if err != nil {
		// The span records that the publish failed and not why. The broker's
		// error text quotes the subject, and a subject is derived from the
		// event type today but is the sort of thing that grows a tenant or an
		// id in it later.
		span.SetStatus(codes.Error, "publish failed")
		return fmt.Errorf("publish %s to nats: %w", event.Type, err)
	}
	return nil
}

// msgIDFor is the broker's deduplication key for one delivery of an event.
func msgIDFor(event messaging.Event) string {
	if event.ReplayGeneration == 0 {
		return event.ID.String()
	}
	return fmt.Sprintf("%s/replay-%d", event.ID, event.ReplayGeneration)
}

// subjectFor maps an event type to a subject.
//
// The type is already dot-separated and hierarchical, which is exactly what
// NATS subjects are, so "user.created.v1" becomes "events.user.created.v1" and
// a subscriber can ask for "events.user.>" without a mapping table.
func subjectFor(prefix, eventType string) string {
	return prefix + "." + eventType
}

// Subscription pulls events from JetStream and hands them to a processor.
type Subscription struct {
	// tracer is never nil; see SetTracer.
	tracer telemetry.TracerProvider

	conn     jetstream.Consumer
	logger   *slog.Logger
	inFlight sync.WaitGroup
}

// Processor handles one delivered event.
//
// Returning nil means the message may be acknowledged: it was handled, already
// handled, or parked. A non-nil error leaves it unacknowledged for redelivery.
type Processor interface {
	Process(ctx context.Context, event messaging.Event) error
}

// Subscribe creates or updates the durable consumer and returns a handle.
//
// filters takes one or more event types. Passing more than one is how a
// single worker process runs one durable consumer — one dedupe log, one
// in-flight drain on shutdown — across several unrelated event types (the
// welcome email and the invitation email, say) instead of standing up a
// separate consumer per type: JetStream's FilterSubjects accepts multiple
// subjects on one consumer, and the delivered messages still carry their own
// Event.Type for a Processor to dispatch on.
func Subscribe(ctx context.Context, conn *Conn, logger *slog.Logger, filters ...string) (*Subscription, error) {
	subjects := make([]string, len(filters))
	for i, filter := range filters {
		subjects[i] = conn.cfg.SubjectPrefix + "." + filter
	}

	consumer, err := conn.stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:   conn.cfg.ConsumerName,
		AckPolicy: jetstream.AckExplicitPolicy,
		AckWait:   conn.cfg.AckWait,
		// Start at the first message the stream still holds. The dedupe log
		// makes replaying old messages harmless, and the alternative — start
		// at "now" — silently drops everything published while the consumer
		// did not exist.
		DeliverPolicy:  jetstream.DeliverAllPolicy,
		MaxDeliver:     conn.cfg.MaxDeliver,
		FilterSubjects: subjects,
	})
	if err != nil {
		return nil, fmt.Errorf("create consumer %s: %w", conn.cfg.ConsumerName, err)
	}

	logger.InfoContext(ctx, "subscribed to nats",
		slog.String("consumer", conn.cfg.ConsumerName),
		slog.Any("subjects", subjects))

	return &Subscription{conn: consumer, logger: logger, tracer: telemetry.NoopTracerProvider()}, nil
}

// Run consumes until ctx is cancelled, then waits for in-flight messages.
//
// The wait is what makes a deploy safe: a message being handled when SIGTERM
// arrives finishes and is acknowledged, rather than being abandoned
// mid-transaction and redelivered.
func (s *Subscription) Run(ctx context.Context, processor Processor) error {
	consumeCtx, err := s.conn.Consume(func(msg jetstream.Msg) {
		s.inFlight.Add(1)
		defer s.inFlight.Done()

		event, err := eventFromMsg(msg)
		if err != nil {
			// An undecodable message cannot improve with redelivery, and
			// leaving it unacknowledged would block nothing but fill the
			// redelivery queue forever. Terminate takes it out of rotation
			// and leaves it in the stream for inspection.
			s.logger.ErrorContext(ctx, "discarding undecodable message",
				slog.String("subject", msg.Subject()),
				slog.Any("error", err))
			if termErr := msg.Term(); termErr != nil {
				s.logger.ErrorContext(ctx, "terminating message failed",
					slog.Any("error", termErr))
			}
			return
		}

		msgCtx, span := startConsumeSpan(ctx, s.tracer, event)
		defer span.End()

		if err := processor.Process(msgCtx, event); err != nil {
			// Unacknowledged on purpose: the processor only returns an error
			// when it could neither handle nor park the event, which means
			// redelivery is the correct outcome.
			span.SetStatus(codes.Error, "left for redelivery")
			s.logger.ErrorContext(msgCtx, "message left for redelivery",
				slog.String("event_id", event.ID.String()),
				slog.String("event_type", event.Type),
				slog.Any("error", err))
			if nakErr := msg.NakWithDelay(time.Second); nakErr != nil {
				s.logger.ErrorContext(ctx, "nak failed", slog.Any("error", nakErr))
			}
			return
		}

		if err := msg.Ack(); err != nil {
			// The work is committed; a lost ack costs one redelivery, which
			// the dedupe log absorbs.
			s.logger.WarnContext(ctx, "ack failed; the message will be redelivered",
				slog.String("event_id", event.ID.String()),
				slog.Any("error", err))
		}
	})
	if err != nil {
		return fmt.Errorf("start consuming: %w", err)
	}

	<-ctx.Done()

	// Stop pulling first, then wait for what is already in hand.
	consumeCtx.Stop()
	s.inFlight.Wait()
	s.logger.InfoContext(context.WithoutCancel(ctx), "nats subscription drained")
	return nil
}

// eventFromMsg rebuilds the envelope from headers.
func eventFromMsg(msg jetstream.Msg) (messaging.Event, error) {
	header := msg.Headers()

	id, err := uuid.Parse(header.Get(headerEventID))
	if err != nil {
		return messaging.Event{}, fmt.Errorf("message has no usable %s header: %w", headerEventID, err)
	}

	eventType := header.Get(headerEventType)
	if err := messaging.ValidateType(eventType); err != nil {
		return messaging.Event{}, err
	}

	occurredAt := time.Time{}
	if raw := header.Get(headerOccurredAt); raw != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			occurredAt = parsed
		}
	}

	payload := msg.Data()
	if !json.Valid(payload) {
		return messaging.Event{}, errors.New("payload is not valid json")
	}

	return messaging.Event{
		ID:          id,
		Type:        eventType,
		OccurredAt:  occurredAt,
		TraceParent: header.Get(headerTraceParent),
		Payload:     payload,
	}, nil
}

// SetTracer attaches tracing. Passing nil restores the no-op provider, so a
// caller that has not configured tracing cannot leave a nil interface on the
// delivery path, where a panic costs every message rather than one request.
func (s *Subscription) SetTracer(tp telemetry.TracerProvider) {
	if tp == nil {
		tp = telemetry.NoopTracerProvider()
	}
	s.tracer = tp
}
