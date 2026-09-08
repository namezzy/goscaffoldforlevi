// Command api serves the HTTP API.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// main only decides the exit code. The work is in start() so that its
	// deferred cleanup actually runs: os.Exit skips deferred calls, so calling
	// it in the same function as a defer silently loses the cleanup.
	os.Exit(start())
}

func start() int {
	// signal.NotifyContext gives the shutdown sequence a context to react to.
	// A second signal is not intercepted: an operator who sends SIGTERM twice
	// is asking for the process to die now, and the default handler obliges.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		// Startup errors happen before the logger exists, so this goes to
		// stderr directly. The exit code is what the orchestrator reads.
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		return 1
	}
	return 0
}
