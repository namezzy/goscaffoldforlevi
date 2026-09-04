// Package redis owns the cache client.
//
// Redis is optional by default (see ADR-0006): a cache outage degrades
// performance, it does not take the service down. The decision of what to do
// when it is unavailable belongs to the caller, not to this constructor, so
// NewClient always reports a connection failure as an error.
package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"github.com/namezzy/goscaffoldforlevi/internal/config"
)

// NewClient creates a client and verifies connectivity.
//
// A failed ping closes the client and returns an error; nothing is leaked.
func NewClient(ctx context.Context, cfg config.Redis) (*goredis.Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:        cfg.Addr,
		Password:    cfg.Password.Reveal(),
		DB:          cfg.DB,
		PoolSize:    cfg.PoolSize,
		DialTimeout: cfg.DialTimeout,
		ReadTimeout: cfg.ReadTimeout,
		// go-redis defaults WriteTimeout to ReadTimeout; stating it keeps the
		// two from drifting apart when only one is tuned.
		WriteTimeout: cfg.ReadTimeout,
	})

	pingCtx, cancel := context.WithTimeout(ctx, cfg.DialTimeout)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return client, nil
}
