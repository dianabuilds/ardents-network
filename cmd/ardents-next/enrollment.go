package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
)

func runEnrollment(ctx context.Context, args []string, out io.Writer) int {
	if len(args) != 3 || (args[0] != "verify" && args[0] != "verify-headless") {
		return enrollmentReport(out, "invalid-input", 2)
	}
	program, err := os.Executable()
	if err != nil {
		return enrollmentReport(out, "unavailable", 1)
	}
	scope := enrollment.General
	if args[0] == "verify-headless" {
		scope = enrollment.Headless
	}
	bundle, err := enrollment.Verify(ctx, enrollment.Request{BundleRoot: args[1], ExecutablePath: program, ManifestSHA256: args[2], Scope: scope})
	if err != nil {
		outcome, code := "unavailable", 1
		switch {
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			outcome, code = "canceled", 130
		case errors.Is(err, enrollment.ErrInput):
			outcome, code = "invalid-input", 2
		case errors.Is(err, enrollment.ErrPin):
			outcome = "pin-mismatch"
		case errors.Is(err, enrollment.ErrLegacyEnrollmentDescriptor):
			outcome = "retired-descriptor"
		case errors.Is(err, enrollment.ErrInventory):
			outcome = "invalid-inventory"
		case errors.Is(err, enrollment.ErrBinding):
			outcome = "binding-mismatch"
		}
		return enrollmentReport(out, outcome, code)
	}
	facts, _ := bundle.Facts()
	programBytes, ok := bundle.File(facts.Artifact)
	if !ok {
		return enrollmentReport(out, "unavailable", 1)
	}
	defer clear(programBytes)
	// Report the actual checked scope without projecting artifact contents,
	// descriptor identities or any later authority as a successful launch.
	report := struct {
		Outcome      string `json:"outcome"`
		Headless     bool   `json:"headless,omitempty"`
		Protected    bool   `json:"protected,omitempty"`
		Files        int    `json:"files"`
		ProgramBytes int    `json:"program_bytes"`
	}{"verified-initial-bundle", facts.Headless, facts.Protected, len(bundle.Names()), len(programBytes)}
	if ctx.Err() != nil {
		return enrollmentReport(out, "canceled", 130)
	}
	if json.NewEncoder(out).Encode(report) != nil {
		return 2
	}
	return 0
}

func enrollmentReport(out io.Writer, outcome string, code int) int {
	if json.NewEncoder(out).Encode(struct {
		Outcome string `json:"outcome"`
	}{outcome}) != nil {
		return 2
	}
	return code
}
