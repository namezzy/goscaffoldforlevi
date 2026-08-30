// Package postgres owns the connection pool and transaction management.
//
// It deliberately contains no business vocabulary: repositories live next to
// the feature they serve (see ADR-0002) and use this package for plumbing only.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/go-enterprise/internal/config"
)

// NewPool creates and verifies a connection pool.
//
// The context bounds the whole operation, and the pool is closed again if the
// initial ping fails, so a failed startup leaves no sockets behind.
func NewPool(ctx context.Context, cfg config.Database) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL.Reveal())
	if err != nil {
		// The URL contains credentials, so the driver error is not propagated
		// verbatim: pgx includes the DSN in some parse failures.
		return nil, fmt.Errorf("parse database url: invalid connection string")
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod
	poolCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
