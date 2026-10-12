//go:build integration

package incidentpg_test

import (
	"context"
	"testing"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/incident/incidentpg"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/pgtest"
)

// TestFreshPollStateIsReadable is the regression test for the watcher never
// opening a single incident: the exception_center migration seeded the
// checkpoint with '-infinity', which pgx cannot scan into a time.Time, so
// every poll on a freshly migrated database failed before reading any dead
// letter. The watcher's unit tests use a fake store and could not see it.
func TestFreshPollStateIsReadable(t *testing.T) {
	pg := pgtest.Start(t)
	store := incidentpg.New(pg.Pool)

	got, err := store.LockPollState(context.Background())
	if err != nil {
		t.Fatalf("LockPollState() on a fresh database = %v, want nil", err)
	}
	// Must sort before any real dead letter, or the first poll skips it.
	if !got.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("LockPollState() = %v, want a checkpoint before any real failure", got)
	}
}
