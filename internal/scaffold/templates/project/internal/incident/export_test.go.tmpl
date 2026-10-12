package incident

import (
	"context"
	"io"
	"log/slog"
)

// PollOnceForTest runs one iteration of the watcher's dead-letter poll —
// the same unexported pollOnce Run's ticker calls — so watcher_test.go can
// drive it deterministically instead of racing a real ticker.
//
// It lives in an _test.go file so it exists only while this package's own
// tests are compiled: the production API is unchanged, and no other package
// can reach it — matching messaging's SetSleepForTest.
func PollOnceForTest(w *Watcher, ctx context.Context) error {
	_, err := w.pollOnce(ctx)
	return err
}

// ResolvePendingReplaysForTest runs one sweep of the watcher's replay-result
// resolver.
func ResolvePendingReplaysForTest(w *Watcher, ctx context.Context) {
	w.resolvePendingReplays(ctx)
}

// DiscardLoggerForTest is a *slog.Logger that writes nowhere, for tests that
// need one to build a Watcher but assert on its state, not its log output.
func DiscardLoggerForTest() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
