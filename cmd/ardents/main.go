package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	processdiag "github.com/dianabuilds/ardents-network/internal/diagnostics/process"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer stop()
	if err := processdiag.Run(ctx, os.Getenv("ARDENTS_DEBUG_SOCKET"), func(workCtx context.Context) error { return run(workCtx, os.Args[1:], os.Stdout) }); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
