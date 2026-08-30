//go:build integration

// Package pgtest starts a throwaway PostgreSQL instance for tests.
//
// Tests that touch SQL run against real PostgreSQL rather than a mock or
// SQLite. The behaviour worth testing here is the behaviour a mock would have
// to invent: constraint violations, transaction isolation, the exact error
// codes the repository layer translates. A test that passes against a fake and
// fails against the database has cost more than it saved.
//
// The integration build tag keeps `go test ./...` fast and Docker-free.
package pgtest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// Container is a running PostgreSQL instance with all migrations applied.
type Container struct {
	URL  string
	Pool *pgxpool.Pool
}

// Start launches PostgreSQL, applies the migrations, and registers cleanup.
//
// Applying the real migrations rather than a hand-written schema is the point:
// every test run also verifies that the migrations produce the schema the code
// expects, which is otherwise a failure that only surfaces during a deploy.
func Start(t *testing.T) *Container {
	t.Helper()

	if _, err := os.Stat("/var/run/docker.sock"); err != nil && os.Getenv("DOCKER_HOST") == "" {
		t.Skip("SKIPPED: Docker is not available")
	}

	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := container.Terminate(stopCtx); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	applyMigrations(t, url)

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return &Container{URL: url, Pool: pool}
}

func applyMigrations(t *testing.T, url string) {
	t.Helper()

	// The migrations directory is located from the module root rather than
	// with a relative path, because the working directory of a test is the
	// package under test and would differ for every caller.
	dir := filepath.Join(moduleRoot(t), "migrations")

	m, err := migrate.New("file://"+dir, url)
	if err != nil {
		t.Fatalf("open migrations: %v", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("apply migrations: %v", err)
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the working directory")
		}
		dir = parent
	}
}
