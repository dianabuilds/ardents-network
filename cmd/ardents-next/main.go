package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/measurement"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/telemetry"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	work := func() int { return run(ctx, os.Args[1:], os.Stdout, os.Stderr) }
	code := 2
	switch os.Getenv("ARDENTS_LOCAL_METRICS") {
	case "":
		code = work()
	case "1":
		endpoint := os.Getenv("ARDENTS_METRICS_OTLP_ENDPOINT")
		observer, err := telemetry.Open(endpoint)
		if err != nil {
			fmt.Fprintln(os.Stderr, "measurement exporter configuration unavailable")
			break
		}
		var exporter measurement.Exporter
		if observer != nil {
			exporter = observer
		}
		code = measurement.Run(os.Stderr, work, exporter)
	default:
		fmt.Fprintln(os.Stderr, "invalid local measurement configuration")
	}
	stop()
	os.Exit(code)
}
