//go:build integration

package incidentpg_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/namezzy/goscaffoldforlevi/internal/incident/incidentpg"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/pgtest"
)

// insertDeadLetter writes a dead letter straight to the table. The watcher
// only ever reads these columns, and going through messagingpg would drag the
// outbox and its own invariants into a test about triage ordering.
func insertDeadLetter(t *testing.T, pool *pgxpool.Pool, failedAt time.Time) string {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(context.Background(), `
		INSERT INTO dead_letter_events
			(id, event_id, event_type, payload, consumer, attempts, last_error, failed_at)
		VALUES ($1, $2, 'user.created.v1', '{}'::jsonb, 'welcome-email', 5, 'boom', $3)`,
		id, uuid.Must(uuid.NewV7()), failedAt)
	if err != nil {
		t.Fatalf("inserting dead letter: %v", err)
	}
	return id.String()
}

// TestADeadLetterThatCommitsLateIsStillClaimed is the regression test for the
// watcher silently dropping failures. Triage used to ask for dead letters
// with failed_at after a checkpoint it had already advanced. Commit order
// does not follow failed_at order — a transaction can stamp failed_at and
// commit after a later failure has already moved the checkpoint past it — and
// that row then became invisible forever, because nothing ever looked back.
func TestADeadLetterThatCommitsLateIsStillClaimed(t *testing.T) {
	pg := pgtest.Start(t)
	ctx := context.Background()
	store := incidentpg.New(pg.Pool)

	// The late writer stamps failed_at first but has not committed yet.
	late := time.Now().UTC().Add(-time.Minute)

	early := insertDeadLetter(t, pg.Pool, late.Add(30*time.Second))
	claimAndMark(t, store, early)

	// Now it commits, with a failed_at older than the one already triaged.
	lateID := insertDeadLetter(t, pg.Pool, late)

	got, ok, err := store.ClaimUntriaged(ctx)
	if err != nil || !ok {
		t.Fatalf("ClaimUntriaged() = ok %v, err %v; want the late dead letter", ok, err)
	}
	if got.ID != lateID {
		t.Errorf("ClaimUntriaged() = %s, want the late-committing %s", got.ID, lateID)
	}
}

// TestClaimingSkipsARowAnotherWatcherHolds covers running more than one
// replica: the second must move on to the next dead letter rather than block
// behind the first, or added replicas buy no throughput at all.
func TestClaimingSkipsARowAnotherWatcherHolds(t *testing.T) {
	pg := pgtest.Start(t)
	ctx := context.Background()
	store := incidentpg.New(pg.Pool)

	now := time.Now().UTC()
	first := insertDeadLetter(t, pg.Pool, now.Add(-time.Minute))
	second := insertDeadLetter(t, pg.Pool, now)

	// Holding the first row for the whole of this transaction, then asking
	// again on another connection from inside it.
	err := postgres.NewTxManager(pg.Pool).WithTx(ctx, func(txCtx context.Context) error {
		occ, ok, err := store.ClaimUntriaged(txCtx)
		if err != nil || !ok || occ.ID != first {
			t.Fatalf("first ClaimUntriaged() = %+v, ok %v, err %v; want %s", occ, ok, err, first)
		}

		other, ok, err := store.ClaimUntriaged(ctx)
		if err != nil || !ok {
			t.Fatalf("second ClaimUntriaged() = ok %v, err %v; want the next dead letter", ok, err)
		}
		if other.ID != second {
			t.Errorf("second ClaimUntriaged() = %s, want %s rather than waiting on the held row", other.ID, second)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("holding transaction: %v", err)
	}
}

// TestAFreshFailureOnTheSameRowIsTriagedAgain is the price of the per-row
// claim: dead_letter_events reuses one row per (event_id, consumer), so a
// retry that fails again overwrites the row the watcher already triaged.
// RecordDeadLetter clears the claim for exactly this reason; without that,
// every repeat failure of an event that dead-lettered once would be invisible
// to the exception center.
func TestAFreshFailureOnTheSameRowIsTriagedAgain(t *testing.T) {
	pg := pgtest.Start(t)
	ctx := context.Background()
	store := incidentpg.New(pg.Pool)

	id := insertDeadLetter(t, pg.Pool, time.Now().UTC().Add(-time.Minute))
	claimAndMark(t, store, id)

	if _, ok, err := store.ClaimUntriaged(ctx); ok || err != nil {
		t.Fatalf("ClaimUntriaged() after triage = ok %v, err %v; want nothing left", ok, err)
	}

	// What RecordDeadLetter's ON CONFLICT does when the replay fails again.
	if _, err := pg.Pool.Exec(ctx, `
		UPDATE dead_letter_events
		SET attempts = attempts + 1, last_error = 'boom again', failed_at = now(),
		    replayed_at = NULL, incident_processed_at = NULL
		WHERE id = $1`, id); err != nil {
		t.Fatalf("re-failing the dead letter: %v", err)
	}

	got, ok, err := store.ClaimUntriaged(ctx)
	if err != nil || !ok || got.ID != id {
		t.Errorf("ClaimUntriaged() after a fresh failure = %+v, ok %v, err %v; want %s back in triage", got, ok, err, id)
	}
}

func claimAndMark(t *testing.T, store *incidentpg.Store, id string) {
	t.Helper()
	ctx := context.Background()
	occ, ok, err := store.ClaimUntriaged(ctx)
	if err != nil || !ok || occ.ID != id {
		t.Fatalf("ClaimUntriaged() = %+v, ok %v, err %v; want %s", occ, ok, err, id)
	}
	if err := store.MarkTriaged(ctx, id); err != nil {
		t.Fatalf("MarkTriaged(%s) = %v, want nil", id, err)
	}
}
