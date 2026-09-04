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

	"github.com/namezzy/goscaffoldforlevi/internal/config"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging/messagingpg"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging/natsmq"
	"github.com/namezzy/goscaffoldforlevi/internal/notify"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/telemetry"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
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

	consumer := messaging.NewConsumer(
		notify.NewWelcomeHandler(notify.NewLogSender(logger), logger),
		store,
		consumerConfig(cfg.Messaging),
		logger,
	)
	consumer.SetObserver(metrics)

	// The consumer subscribes to the one event it handles rather than to
	// everything. Filtering at the broker means an unrelated event never
	// becomes this consumer's problem, and never appears in its dedupe log.
	subscription, err := natsmq.Subscribe(ctx, conn, user.EventUserCreated, logger)
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	subscription.SetTracer(tracer)

	// Both halves stop on the same context and are waited for. Nothing here
	// depends on finishing in time: an event the relay did not publish is
	// still in the table, and a message the consumer did not acknowledge will
	// be redelivered. The wait exists to make the common case clean, not to
	// make correctness depend on it.
	var wg sync.WaitGroup
	errs := make(chan error, 2)

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
