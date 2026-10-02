package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if (len(args) != 3 && len(args) != 4) || args[0] != "inspect-permission" {
		fmt.Fprintln(diagnostic, "usage: ardents-next inspect-permission <permission-file> <facts-file> [collector]")
		return 2
	}
	endpoint := ""
	if len(args) == 4 {
		endpoint = args[3]
	}
	if collectorEndpoint(endpoint) != nil {
		return report(out, admission.InvalidInput, 2)
	}
	raw, err := readBounded(args[1], 228)
	if err != nil {
		return report(out, admission.InvalidInput, 2)
	}
	defer clear(raw)
	config, err := readBounded(args[2], 4096)
	if err != nil {
		return report(out, admission.InvalidInput, 2)
	}
	defer clear(config)
	facts, err := decodeFacts(config)
	if err != nil {
		return report(out, admission.InvalidInput, 2)
	}
	observation, telemetryErr := newObservation(endpoint)
	result := observation.inspect(ctx, raw, facts)
	ok := observation.close()
	telemetry := "completed"
	if telemetryErr != nil || !ok {
		telemetry = "unavailable"
	}
	if endpoint == "" {
		telemetry = "local-only"
	}
	_ = json.NewEncoder(diagnostic).Encode(struct {
		Event     string            `json:"event"`
		Outcome   admission.Outcome `json:"outcome"`
		Telemetry string            `json:"telemetry"`
	}{"permission-inspected", result, telemetry})
	code := 1
	if result == admission.Accepted {
		code = 0
	}
	if result == admission.Canceled {
		code = 130
	}
	return report(out, result, code)
}

func report(out io.Writer, result admission.Outcome, code int) int {
	if json.NewEncoder(out).Encode(struct {
		Outcome admission.Outcome `json:"outcome"`
	}{result}) != nil {
		return 2
	}
	return code
}
