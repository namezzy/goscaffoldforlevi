package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"

	"github.com/namezzy/goscaffoldforlevi/internal/config"
	"github.com/namezzy/goscaffoldforlevi/internal/httpx"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/ratelimit/ratelimithttp"
	"github.com/namezzy/goscaffoldforlevi/internal/redis"
	"github.com/namezzy/goscaffoldforlevi/internal/telemetry"
)

// run wires the application and serves until ctx is cancelled.
//
// It lives in package main rather than in a bootstrap package (ADR-0002): the
// composition root is the one place allowed to know about everything, and
// giving it its own package only adds an import edge without adding a
// boundary. It is a function rather than inline main() code so that
// main_test.go can drive a real startup and shutdown.
func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		// The logger does not exist yet, so this error must be self-describing.
		return fmt.Errorf("load config: %w", err)
	}

	logger := telemetry.NewLogger(cfg.App, telemetry.LoggerOptions{Level: cfg.App.LogLevel})
	logger.InfoContext(ctx, "starting",
		slog.String("addr", cfg.HTTP.Addr()),
		slog.Bool("otel_enabled", cfg.OTel.Enabled),
		slog.Bool("metrics_enabled", cfg.Metrics.Enabled),
	)

	// Telemetry is built before any dependency, so that the pool and the cache
	// can be instrumented as they are created rather than retro-fitted.
	tracer, err := telemetry.NewTracerProvider(ctx, cfg.App, cfg.OTel)
	if err != nil {
		return fmt.Errorf("build tracer: %w", err)
	}
	defer func() {
		// A separate budget: ctx is already cancelled by the time this runs,
		// and flushing the last spans of a shutdown is exactly when they
		// matter most.
		flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := tracer.Shutdown(flushCtx); err != nil {
			logger.Warn("tracer shutdown failed", slog.Any("error", err))
		}
	}()

	// Set globally so that libraries which propagate on their own — the OTLP
	// exporter's own gRPC client included — agree with the middleware about
	// the wire format.
	otel.SetTextMapPropagator(telemetry.Propagator())

	metrics := telemetry.NewMetrics()
	metricsServer, metricsAddr, err := telemetry.NewMetricsServer(ctx, cfg.Metrics, metrics)
	if err != nil {
		return fmt.Errorf("start metrics endpoint: %w", err)
	}
	if metricsServer != nil {
		logger.InfoContext(ctx, "metrics listening",
			slog.String("addr", metricsAddr), slog.String("path", cfg.Metrics.Path))
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

	// Dependencies are opened in dependency order and closed in reverse, so a
	// failure part-way through never leaves a connection pool behind.
	pool, err := postgres.NewPool(ctx, cfg.Database, postgres.WithTracing(tracer))
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()

	// Pool saturation is the first thing to look at when latency rises and
	// nothing else changed, and it is invisible from inside a query timing.
	metrics.RegisterPoolCollector(poolStats{pool})

	txManager := postgres.NewTxManager(pool)

	var cache *goredis.Client
	cache, err = redis.NewClient(ctx, cfg.Redis)
	switch {
	case err != nil && cfg.Redis.Required:
		return fmt.Errorf("connect redis: %w", err)
	case err != nil:
		// Degraded, not dead: the cache is optional by configuration, so the
		// service starts and the readiness body reports the component.
		logger.WarnContext(ctx, "redis unavailable, continuing without cache",
			slog.Any("error", err))
		cache = nil
	default:
		cache.AddHook(redis.NewHook(tracer, metrics))
		defer func() { _ = cache.Close() }()
	}

	health := httpx.NewHealth(logger, cfg.App.Version, checkers(pool, cache, cfg.Redis.Required)...)

	features, err := buildFeatures(cfg, pool, cache, txManager, logger, metrics)
	if err != nil {
		return err
	}

	handler := httpx.NewRouter(httpx.RouterDeps{
		Config:        cfg,
		Logger:        logger,
		Health:        health,
		Register:      features.register,
		RateLimitByIP: ratelimithttp.ByIP(features.guard, cfg.HTTP.TrustedProxyCount, logger),
		Tracer:        tracer,
		Metrics:       metrics,
	})

	server := httpx.NewServer(cfg.HTTP, handler)

	return serve(ctx, cfg.HTTP, logger, server, health)
}

// checkers builds the readiness probes for the dependencies that exist.
func checkers(pool *pgxpool.Pool, cache *goredis.Client, redisRequired bool) []httpx.Checker {
	list := []httpx.Checker{
		httpx.CheckFunc{
			ComponentName: "postgres",
			IsRequired:    true,
			Probe: func(ctx context.Context) error {
				return postgres.HealthCheck(ctx, pool)
			},
		},
	}

	if cache != nil {
		list = append(list, httpx.CheckFunc{
			ComponentName: "redis",
			IsRequired:    redisRequired,
			Probe: func(ctx context.Context) error {
				return cache.Ping(ctx).Err()
			},
		})
	}

	return list
}

// serve runs the server and performs the shutdown sequence described in
// docs/architecture.md §8.
//
// The order matters more than the code does:
//
//  1. Mark unready and wait. The load balancer needs time to notice before the
//     socket closes, otherwise clients see connection resets rather than a
//     clean failover. This is the step people skip, and it is the reason
//     "graceful" deploys still drop requests.
//  2. Stop accepting and drain in-flight requests.
//  3. Close dependencies (deferred by the caller), after the last handler that
//     could use them has returned.
func serve(
	ctx context.Context,
	cfg config.HTTP,
	logger *slog.Logger,
	server *http.Server,
	health *httpx.Health,
) error {
	serverErr := make(chan error, 1)
	go func() {
		logger.InfoContext(ctx, "listening", slog.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	// The shutdown budget is bounded independently of ctx, which is already
	// cancelled at this point.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
	defer cancel()

	health.StartDraining()
	logger.Info("draining", slog.Duration("delay", cfg.ReadinessDrainDelay))

	select {
	case <-time.After(cfg.ReadinessDrainDelay):
	case <-shutdownCtx.Done():
	}

	if err := server.Shutdown(shutdownCtx); err != nil {
		// Force-close so that the process exits instead of hanging on a
		// connection that refuses to finish.
		_ = server.Close()
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	logger.Info("shutdown complete")
	return nil
}

// poolStats adapts the pgx pool to the telemetry package's collector, so that
// telemetry does not import pgx and pgx does not import Prometheus.
type poolStats struct{ pool *pgxpool.Pool }

func (p poolStats) TotalConns() int32    { return p.pool.Stat().TotalConns() }
func (p poolStats) IdleConns() int32     { return p.pool.Stat().IdleConns() }
func (p poolStats) AcquiredConns() int32 { return p.pool.Stat().AcquiredConns() }
