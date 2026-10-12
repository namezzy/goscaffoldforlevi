package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/otel"

	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/audit/auditpg"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/buildinfo"
	"github.com/namezzy/goscaffoldforlevi/internal/config"
	"github.com/namezzy/goscaffoldforlevi/internal/incident"
	"github.com/namezzy/goscaffoldforlevi/internal/incident/incidentpg"
	"github.com/namezzy/goscaffoldforlevi/internal/invite"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging/messagingpg"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging/natsmq"
	"github.com/namezzy/goscaffoldforlevi/internal/notification"
	"github.com/namezzy/goscaffoldforlevi/internal/notification/notificationpg"
	"github.com/namezzy/goscaffoldforlevi/internal/notify"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/telemetry"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
	"github.com/namezzy/goscaffoldforlevi/internal/user/userpg"
)

// run starts the relay and the consumer and serves until ctx is cancelled.
func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := telemetry.NewLogger(cfg.App, telemetry.LoggerOptions{Level: cfg.App.LogLevel})

	if !cfg.Messaging.Enabled {
		// Refusing to start is the useful behaviour. A worker that idles
		// politely with messaging disabled looks healthy in every dashboard
		// while delivering nothing, and the misconfiguration is found by
		// someone wondering where their events went.
		return errors.New(
			"MESSAGING_ENABLED is false: the worker has nothing to do. " +
				"Set MESSAGING_ENABLED=true, or do not deploy this process")
	}

	logger.InfoContext(ctx, "worker starting",
		slog.String("commit", buildinfo.Commit),
		slog.String("build_time", buildinfo.BuildTime),
		slog.String("consumer", cfg.Messaging.ConsumerName),
		slog.Any("nats_urls", cfg.Messaging.NATSURLs),
		slog.Bool("otel_enabled", cfg.OTel.Enabled),
		slog.Bool("metrics_enabled", cfg.Metrics.Enabled))

	tracer, err := telemetry.NewTracerProvider(ctx, cfg.App, cfg.OTel)
	if err != nil {
		return fmt.Errorf("build tracer: %w", err)
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := tracer.Shutdown(flushCtx); err != nil {
			logger.Warn("tracer shutdown failed", slog.Any("error", err))
		}
	}()
	otel.SetTextMapPropagator(telemetry.Propagator())

	metrics := telemetry.NewMetrics()
	metricsServer, metricsAddr, err := telemetry.NewMetricsServer(ctx, workerMetricsConfig(cfg.Metrics), metrics)
	if err != nil {
		return fmt.Errorf("start metrics endpoint: %w", err)
	}
	if metricsServer != nil {
		logger.InfoContext(ctx, "metrics listening", slog.String("addr", metricsAddr))
		go func() {
			if err := metricsServer.Serve(); err != nil {
				logger.Error("metrics endpoint failed", slog.Any("error", err))
			}
		}()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_ = metricsServer.Shutdown(shutdownCtx)
		}()
	}

	pool, err := postgres.NewPool(ctx, cfg.Database, postgres.WithTracing(tracer))
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()

	store := messagingpg.NewStore(pool, postgres.NewTxManager(pool))

	// Built here, ahead of the consumer, since the incident alert handler
	// below (conditionally added to its dispatch list) and the incident
	// watcher both need them.
	auditService := audit.NewService(auditpg.New(pool))
	notificationService := notification.NewService(notificationpg.New(pool))
	userRepo := userpg.New(pool)

	conn, err := natsmq.Connect(ctx, natsConfig(cfg.Messaging), logger)
	if err != nil {
		return fmt.Errorf("connect nats: %w", err)
	}
	defer conn.Close()

	publisher := natsmq.NewPublisher(conn)
	publisher.SetTracer(tracer)

	relay := messaging.NewRelay(
		store,
		publisher,
		relayConfig(cfg.Messaging),
		logger,
	)
	relay.SetObserver(metrics)

	sender := notify.NewLogSender(logger)
	handlers := []messaging.Handler{
		notify.NewWelcomeHandler(sender, logger),
		notify.NewInvitationHandler(
			sender, cfg.Invite.AcceptURLBase,
			// Must be built from the same secret cmd/api's invite.NewService
			// used to encrypt — see invite.NewTokenCipher's doc comment.
			invite.NewTokenCipher(cfg.Auth.JWTSecret.Reveal()), logger,
		),
		notify.NewPasswordResetHandler(
			sender, cfg.Auth.PasswordResetURLBase,
			// Must be built from the same secret cmd/api's auth.NewService
			// used to encrypt — see auth.NewTokenCipher's doc comment.
			auth.NewTokenCipher(cfg.Auth.JWTSecret.Reveal()), logger,
		),
	}
	// The external alert channel is opt-in: with no address configured, the
	// exception center still notifies every incident:manage holder in-app
	// (incident.Watcher does that directly, not through this dispatch
	// list) — this only adds the email on top of that.
	if cfg.Notify.IncidentAlertEmail != "" {
		handlers = append(handlers, notify.NewIncidentAlertHandler(sender, cfg.Notify.IncidentAlertEmail, logger))
	}
	consumer := messaging.NewConsumer(
		// Dispatched by event type, in the same durable consumer/dedupe log —
		// see notify.DispatchHandler and natsmq.Subscribe's doc comment for
		// why this is one subscription across every event type rather than
		// one per type.
		notify.DispatchHandler(handlers...),
		store,
		consumerConfig(cfg.Messaging),
		logger,
	)
	consumer.SetObserver(metrics)

	// The consumer subscribes to exactly the event types it handles rather
	// than to everything. Filtering at the broker means an unrelated event
	// never becomes this consumer's problem, and never appears in its
	// dedupe log. incident.EventIncidentOpened is subscribed unconditionally
	// (not gated on IncidentAlertEmail): every handler above already ignores
	// an event type that is not its own, so an unconfigured deployment just
	// sees it dispatched to nothing, harmlessly, rather than needing its own
	// separate subscription toggle.
	subscription, err := natsmq.Subscribe(ctx, conn, logger,
		user.EventUserCreated, invite.EventInvitationCreated, auth.EventPasswordResetRequested,
		incident.EventIncidentOpened)
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	subscription.SetTracer(tracer)

	incidentWatcher := incident.NewWatcher(
		incidentpg.New(pool), postgres.NewTxManager(pool), auditService,
		notificationService, userRepo, publisher, incident.DefaultWatcherConfig(), logger,
	)

	// Both halves stop on the same context and are waited for. Nothing here
	// depends on finishing in time: an event the relay did not publish is
	// still in the table, and a message the consumer did not acknowledge will
	// be redelivered. The wait exists to make the common case clean, not to
	// make correctness depend on it.
	var wg sync.WaitGroup
	errs := make(chan error, 3)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := relay.Run(ctx); err != nil {
			errs <- fmt.Errorf("relay: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := subscription.Run(ctx, consumer); err != nil {
			errs <- fmt.Errorf("subscription: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := incidentWatcher.Run(ctx); err != nil {
			errs <- fmt.Errorf("incident watcher: %w", err)
		}
	}()

	<-ctx.Done()
	logger.InfoContext(context.WithoutCancel(ctx), "worker draining")

	drained := make(chan struct{})
	go func() {
		wg.Wait()
		close(drained)
	}()

	select {
	case <-drained:
		logger.Info("worker stopped cleanly")
	case <-time.After(cfg.HTTP.ShutdownTimeout):
		// Not a data loss event, which is why it is a warning rather than a
		// failure: whatever was in flight is unacknowledged or unpublished and
		// will be picked up again.
		logger.Warn("worker drain timed out; in-flight work will be redelivered",
			slog.Duration("timeout", cfg.HTTP.ShutdownTimeout))
	}

	close(errs)
	var runErr error
	for err := range errs {
		runErr = errors.Join(runErr, err)
	}
	return runErr
}

func natsConfig(cfg config.Messaging) natsmq.Config {
	return natsmq.Config{
		URLs:           cfg.NATSURLs,
		StreamName:     cfg.StreamName,
		SubjectPrefix:  cfg.SubjectPrefix,
		ConsumerName:   cfg.ConsumerName,
		ConnectTimeout: cfg.ConnectTimeout,
		PublishTimeout: cfg.PublishTimeout,
		MaxAge:         cfg.StreamMaxAge,
		AckWait:        cfg.AckWait,
		MaxDeliver:     cfg.MaxDeliver,
	}
}

func relayConfig(cfg config.Messaging) messaging.RelayConfig {
	return messaging.RelayConfig{
		PollInterval:       cfg.RelayPollInterval,
		BatchSize:          cfg.RelayBatchSize,
		PublishTimeout:     cfg.PublishTimeout,
		PublishedRetention: cfg.PublishedRetention,
		ProcessedRetention: cfg.ProcessedRetention,
		PruneInterval:      cfg.PruneInterval,
	}
}

func consumerConfig(cfg config.Messaging) messaging.ConsumerConfig {
	return messaging.ConsumerConfig{
		Name:           cfg.ConsumerName,
		HandlerTimeout: cfg.HandlerTimeout,
		MaxAttempts:    cfg.MaxAttempts,
		BaseBackoff:    cfg.BaseBackoff,
		MaxBackoff:     cfg.MaxBackoff,
	}
}

// workerMetricsConfig moves the worker's metrics endpoint off the API's port.
//
// The two processes share one configuration file and, in development, one
// host. Left on the same address the second process to start fails to bind,
// and the failure reads as "metrics endpoint" rather than as "two services on
// one port", which is a confusing way to lose a deployment. Incrementing the
// port keeps both scrapable with no extra variable to forget.
func workerMetricsConfig(cfg config.Metrics) config.Metrics {
	if !cfg.Enabled {
		return cfg
	}
	host, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return cfg
	}
	n, err := strconv.Atoi(port)
	if err != nil || n == 0 {
		return cfg
	}
	cfg.Addr = net.JoinHostPort(host, strconv.Itoa(n+1))
	return cfg
}
