// Command worker delivers outbox events and consumes them.
//
// It is a separate binary from the API because the two have different failure
// modes and different scaling needs: a slow consumer must not make HTTP
// requests slow, and restarting the worker must not drop traffic. They share
// the database and the code, not a process.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	os.Exit(start())
}

func start() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		return 1
	}
	return 0
}
