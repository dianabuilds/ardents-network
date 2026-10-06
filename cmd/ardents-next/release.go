package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// verify-initial requires independently pinned bytes and keeps Release history
// separate. Its output is verification, never installation or live readiness.
func runRelease(ctx context.Context, args []string, out io.Writer) int {
	if len(args) != 5 || args[0] != "verify-initial" {
		return enrollmentReport(out, "invalid-input", 2)
	}
	ref, err := time.Parse(time.RFC3339, args[4])
	if err != nil {
		return enrollmentReport(out, "invalid-input", 2)
	}
	program, err := os.Executable()
	if err != nil {
		return enrollmentReport(out, "release-unavailable", 1)
	}
	bundle, err := enrollment.Verify(ctx, enrollment.Request{BundleRoot: args[1], ExecutablePath: program, ManifestSHA256: args[2], Scope: enrollment.General})
	if err != nil {
		return enrollmentReport(out, "enrollment-refused", 1)
	}
	facts, ok := bundle.Facts()
	if !ok || facts.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		return enrollmentReport(out, "release-incompatible", 1)
	}
	in, ok := initialReleaseInputs(bundle, ref)
	if !ok {
		return enrollmentReport(out, "release-invalid", 1)
	}
	verifier, err := release.Open(args[3])
	if err != nil {
		return enrollmentReport(out, "release-unavailable", 1)
	}
	decision := verifier.Evaluate(ctx, in)
	floors, floorErr := verifier.CurrentFloors(ctx)
	closeErr := verifier.Close()
	if closeErr == nil && (errors.Is(decision.Err(), context.Canceled) || errors.Is(decision.Err(), context.DeadlineExceeded)) {
		return enrollmentReport(out, "release-unavailable", 130)
	}
	if closeErr != nil || floorErr != nil || ctx.Err() != nil {
		return enrollmentReport(out, "release-unavailable", 1)
	}
	code := 1
	if authorization, ok := decision.Authorization(); ok {
		accepted, valid := authorization.AcceptedDecision()
		if !valid {
			return enrollmentReport(out, "release-invalid", 1)
		}
		// The successful report is projected from the private verified fact,
		// rather than treating mutable public Decision fields as authority.
		decision = accepted
		code = 0
	}
	if json.NewEncoder(out).Encode(struct {
		Outcome     release.Outcome `json:"outcome"`
		RootVersion int64           `json:"root_version"`
	}{decision.Outcome, floors.RootVersion}) != nil {
		return 2
	}
	return code
}

func initialReleaseInputs(bundle enrollment.Bundle, ref time.Time) (release.Inputs, bool) {
	f, ok := bundle.Facts()
	if !ok {
		return release.Inputs{}, false
	}
	root, ok := bundle.File(f.TrustedRoot)
	if !ok {
		return release.Inputs{}, false
	}
	artifact, ok := bundle.File(f.Artifact)
	if !ok {
		return release.Inputs{}, false
	}
	files := make(map[string][]byte)
	for _, name := range bundle.MetadataNames() {
		files["https://release.invalid/metadata/"+name], _ = bundle.File(name)
	}
	return release.Inputs{RootBytes: root, Files: files, TargetPath: f.TargetPath, Artifact: artifact, Local: release.LocalEnvironment{Environment: f.Environment, Network: f.Network, Platform: f.Platform, Architecture: runtime.GOARCH, RefTime: ref.UTC()}}, true
}
