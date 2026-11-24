package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
)

func runEnrollmentCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) == 0 {
		return verifyEnrollmentCommand(ctx, args, out)
	}
	var err error
	switch args[0] {
	case "artifact-name":
		if len(args) != 3 {
			fmt.Fprintln(diagnostic, "usage: ardents enrollment artifact-name <command> <platform>")
			return 2
		}
		_, err = fmt.Fprintln(out, enrollment.ExecutableArtifactName(args[1], args[2]))
	case "check-candidate":
		if len(args) != 2 {
			fmt.Fprintln(diagnostic, "usage: ardents enrollment check-candidate <bundle-root>")
			return 2
		}
		_, err = enrollment.ReadCandidate(ctx, args[1])
	case "check-protected-generation":
		err = checkProtectedEnrollmentGeneration(args[1:])
	default:
		return verifyEnrollmentCommand(ctx, args, out)
	}
	if err != nil {
		fmt.Fprintln(diagnostic, err)
		return 2
	}
	return 0
}

func checkProtectedEnrollmentGeneration(args []string) error {
	if len(args) != 4 {
		return fmt.Errorf("usage: ardents enrollment check-protected-generation <static-root> <endpoint> <document-worker> <release-identity>")
	}
	return enrollment.CheckProtectedGenerationFiles(args[0], args[1], args[2], args[3])
}

func verifyEnrollmentCommand(ctx context.Context, args []string, out io.Writer) int {
	if len(args) != 3 || args[0] != "verify" {
		return enrollmentReport(out, "invalid-input", 2)
	}
	program, err := os.Executable()
	if err != nil {
		return enrollmentReport(out, "unavailable", 1)
	}
	bundle, err := enrollment.Verify(ctx, enrollment.Request{BundleRoot: args[1], ExecutablePath: program, ManifestSHA256: args[2]})
	if err != nil {
		outcome, code := "unavailable", 1
		switch {
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			outcome, code = "canceled", 130
		case errors.Is(err, enrollment.ErrInput):
			outcome, code = "invalid-input", 2
		case errors.Is(err, enrollment.ErrPin):
			outcome = "pin-mismatch"
		case errors.Is(err, enrollment.ErrInventory):
			outcome = "invalid-inventory"
		case errors.Is(err, enrollment.ErrBinding):
			outcome = "binding-mismatch"
		}
		return enrollmentReport(out, outcome, code)
	}
	facts, ok := bundle.Facts()
	if !ok || facts.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		return enrollmentReport(out, "binding-mismatch", 1)
	}
	programBytes, ok := bundle.File(facts.Artifact)
	if !ok {
		return enrollmentReport(out, "unavailable", 1)
	}
	defer clear(programBytes)
	// Report the verified inventory and whether it includes protected resources.
	report := struct {
		Outcome      string `json:"outcome"`
		Protected    bool   `json:"protected,omitempty"`
		Files        int    `json:"files"`
		ProgramBytes int    `json:"program_bytes"`
	}{"verified-initial-bundle", facts.Protected, len(bundle.Names()), len(programBytes)}
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
