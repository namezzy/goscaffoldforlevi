package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// rollbackTimeout bounds the rollback issued on the failure path. It is short
// on purpose: if the connection is unhealthy we would rather return the
// original error than block the caller.
const rollbackTimeout = 5 * time.Second

// DBTX is the subset of pgx used by generated queries. Both *pgxpool.Pool and
// pgx.Tx satisfy it, which is what lets a repository run inside or outside a
// transaction without changing its code. It is structurally identical to the
// interface sqlc generates, so values of this type can be passed to it.
type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type txKey struct{}

// TxManager runs a function inside a database transaction.
//
// The transaction travels in the context rather than in every method
// signature. That is a deliberate exception to "explicit over implicit"
// (see ADR-0005): the alternative is threading a pgx.Tx through every
// repository and service method, which would put a driver type into the
// business layer and break the dependency rule. The exception is contained:
// txKey is unexported, so only this package can read or write it.
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type txManager struct {
	pool *pgxpool.Pool
}

// NewTxManager returns a TxManager backed by pool.
func NewTxManager(pool *pgxpool.Pool) TxManager { return &txManager{pool: pool} }

// WithTx begins a transaction, runs fn, and commits if fn returns nil.
//
// Nested calls reuse the outer transaction rather than opening a savepoint:
// partial rollback has no use case here, and nested-transaction semantics are
// a common source of misunderstanding.
//
// A panic inside fn rolls back before unwinding, so a bug never silently
// commits half of an operation.
func (m *txManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if InTx(ctx) {
		return fn(ctx)
	}

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		// Roll back on both the error and the panic path. WithoutCancel keeps
		// a cancelled request context from preventing the rollback.
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
		defer cancel()
		if rbErr := tx.Rollback(rollbackCtx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback: %w", rbErr))
		}
	}()

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	committed = true
	return nil
}

// Querier returns the executor a repository should use: the ambient
// transaction when one is active, otherwise the pool.
//
// This is the only sanctioned way to obtain an executor. Repositories must not
// reach for the pool directly, or they will silently escape transactions.
func Querier(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}

// InTx reports whether ctx carries an active transaction.
func InTx(ctx context.Context) bool {
	_, ok := ctx.Value(txKey{}).(pgx.Tx)
	return ok
}
