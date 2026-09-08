package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/namezzy/goscaffoldforlevi/internal/postgres/db"
)

// HealthCheck verifies that the pool can execute SQL.
//
// It is stronger than pgxpool.Ping: a pool can hold a connection that is open
// but unusable, which is exactly the state a database failover leaves behind,
// and a service that reports ready in that state sends every request into a
// wall. Running a real statement is the only way to tell the difference.
func HealthCheck(ctx context.Context, pool *pgxpool.Pool) error {
	// This also proves that Querier's return type satisfies the interface sqlc
	// generates, which is what lets repositories be transaction-agnostic.
	if _, err := db.New(Querier(ctx, pool)).ReadinessProbe(ctx); err != nil {
		return fmt.Errorf("readiness probe: %w", err)
	}
	return nil
}
