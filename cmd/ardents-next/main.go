package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) > 0 && args[0] == "endpoint" {
		return runInstalledEndpoint(ctx, args[1:], diagnostic)
	}
	if len(args) > 0 && args[0] == "execution" {
		return runExecution(ctx, args[1:], out)
	}
	if len(args) > 0 && args[0] == "installation" {
		return runInstallation(ctx, args[1:], out)
	}
	if len(args) > 0 && args[0] == "release" {
		return runRelease(ctx, args[1:], out)
	}
	if len(args) > 0 && args[0] == "enrollment" {
		return runEnrollment(ctx, args[1:], out)
	}
	if len(args) > 0 && args[0] == "route" {
		return runRoute(ctx, args[1:], out, diagnostic)
	}
	if len(args) > 0 && args[0] == "network" {
		return runNetwork(ctx, args[1:], out)
	}
	if len(args) > 1 && args[0] == "node-identity" && args[1] == "import" {
		return runIssuerProfile(ctx, "node-identity.import", args[2:], out, diagnostic)
	}
	if len(args) > 0 && args[0] == "issuance" {
		return runIssuance(ctx, args[1:], out, diagnostic)
	}
	if len(args) > 0 && args[0] == "admission" {
		return runAdmission(ctx, args[1:], out, diagnostic)
	}
	if len(args) > 0 && args[0] == "hosting" {
		return runHosting(ctx, args[1:], out, diagnostic)
	}
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
