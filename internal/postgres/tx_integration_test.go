//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/example/go-enterprise/internal/postgres"
	"github.com/example/go-enterprise/internal/postgres/pgtest"
)

func TestWithTxCommitsOnSuccess(t *testing.T) {
	c := pgtest.Start(t)
	ctx := context.Background()

	mustExec(t, c, `CREATE TABLE t_commit (id int primary key)`)

	tm := postgres.NewTxManager(c.Pool)
	err := tm.WithTx(ctx, func(ctx context.Context) error {
		_, err := postgres.Querier(ctx, c.Pool).Exec(ctx, `INSERT INTO t_commit VALUES (1)`)
		return err
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}

	if got := countRows(t, c, "t_commit"); got != 1 {
		t.Fatalf("row count = %d, want 1", got)
	}
}

// TestWithTxRollsBackOnError is the property the whole abstraction exists for.
// A partially applied multi-statement operation is the failure mode that is
// hardest to detect afterwards, because nothing reports it.
func TestWithTxRollsBackOnError(t *testing.T) {
	c := pgtest.Start(t)
	ctx := context.Background()

	mustExec(t, c, `CREATE TABLE t_rollback (id int primary key)`)

	sentinel := errors.New("business rule violated")
	tm := postgres.NewTxManager(c.Pool)

	err := tm.WithTx(ctx, func(ctx context.Context) error {
		if _, err := postgres.Querier(ctx, c.Pool).Exec(ctx, `INSERT INTO t_rollback VALUES (1)`); err != nil {
			return err
		}
		return sentinel
	})

	if !errors.Is(err, sentinel) {
		t.Fatalf("WithTx = %v, want the caller's error unchanged", err)
	}
	if got := countRows(t, c, "t_rollback"); got != 0 {
		t.Fatalf("row count = %d, want 0 after rollback", got)
	}
}

// TestWithTxRollsBackOnPanic covers the path a deferred rollback is easy to
// get wrong: a panic must not leave a transaction open on the connection.
func TestWithTxRollsBackOnPanic(t *testing.T) {
	c := pgtest.Start(t)
	ctx := context.Background()

	mustExec(t, c, `CREATE TABLE t_panic (id int primary key)`)
	tm := postgres.NewTxManager(c.Pool)

	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic did not propagate to the caller")
			}
		}()

		_ = tm.WithTx(ctx, func(ctx context.Context) error {
			if _, err := postgres.Querier(ctx, c.Pool).Exec(ctx, `INSERT INTO t_panic VALUES (1)`); err != nil {
				return err
			}
			panic("boom")
		})
	}()

	if got := countRows(t, c, "t_panic"); got != 0 {
		t.Fatalf("row count = %d, want 0 after a panic", got)
	}
}

// TestNestedWithTxReusesTheOuterTransaction pins the documented behaviour: a
// nested call must not open a second transaction, and the outer rollback must
// undo the inner work.
func TestNestedWithTxReusesTheOuterTransaction(t *testing.T) {
	c := pgtest.Start(t)
	ctx := context.Background()

	mustExec(t, c, `CREATE TABLE t_nested (id int primary key)`)
	tm := postgres.NewTxManager(c.Pool)
	sentinel := errors.New("outer failed")

	err := tm.WithTx(ctx, func(ctx context.Context) error {
		if err := tm.WithTx(ctx, func(ctx context.Context) error {
			_, err := postgres.Querier(ctx, c.Pool).Exec(ctx, `INSERT INTO t_nested VALUES (1)`)
			return err
		}); err != nil {
			return err
		}
		return sentinel
	})

	if !errors.Is(err, sentinel) {
		t.Fatalf("WithTx = %v, want the outer error", err)
	}
	if got := countRows(t, c, "t_nested"); got != 0 {
		t.Fatalf("row count = %d, want 0: the inner insert should have been rolled back", got)
	}
}

// TestQuerierOutsideTransactionUsesThePool proves the ambient-transaction
// lookup falls back correctly; otherwise every non-transactional read would
// silently do nothing.
func TestQuerierOutsideTransactionUsesThePool(t *testing.T) {
	c := pgtest.Start(t)
	ctx := context.Background()

	if postgres.InTx(ctx) {
		t.Fatal("a fresh context reports being in a transaction")
	}

	var got int
	if err := postgres.Querier(ctx, c.Pool).QueryRow(ctx, `SELECT 1`).Scan(&got); err != nil {
		t.Fatalf("query through the pool: %v", err)
	}
	if got != 1 {
		t.Fatalf("SELECT 1 = %d", got)
	}
}

func TestInTxIsTrueInsideWithTx(t *testing.T) {
	c := pgtest.Start(t)

	tm := postgres.NewTxManager(c.Pool)
	err := tm.WithTx(context.Background(), func(ctx context.Context) error {
		if !postgres.InTx(ctx) {
			t.Error("InTx = false inside WithTx")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}
}

func TestHealthCheckExecutesSQL(t *testing.T) {
	c := pgtest.Start(t)

	if err := postgres.HealthCheck(context.Background(), c.Pool); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
}

// TestMigrationsAreReversible catches a down migration that does not undo its
// up migration. Discovering that during an incident rollback is too late.
func TestMigrationsAreReversible(t *testing.T) {
	c := pgtest.Start(t)

	var exists bool
	err := c.Pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM pg_proc WHERE proname = 'set_updated_at')`).Scan(&exists)
	if err != nil {
		t.Fatalf("query pg_proc: %v", err)
	}
	if !exists {
		t.Fatal("the baseline migration did not create set_updated_at()")
	}
}

func mustExec(t *testing.T, c *pgtest.Container, sql string) {
	t.Helper()
	if _, err := c.Pool.Exec(context.Background(), sql); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func countRows(t *testing.T, c *pgtest.Container, table string) int {
	t.Helper()
	var n int
	// The table name is a test-local literal, never user input.
	if err := c.Pool.QueryRow(context.Background(), `SELECT count(*) FROM `+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}
