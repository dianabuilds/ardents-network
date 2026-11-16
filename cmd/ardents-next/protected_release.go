package main

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/successor/release/endpoint"
)

// runProtectedRelease authenticates exact Enrollment and Release bytes. Its
// public result is not a reusable authorization, install right or readiness.
func runProtectedRelease(ctx context.Context, args []string, out io.Writer, legacy bool) int {
	if len(args) < 1 || (args[0] != "authenticate-initial" && args[0] != "authenticate-candidate") ||
		(args[0] == "authenticate-initial" && len(args) != 5) ||
		(args[0] == "authenticate-candidate" && len(args) != 4) {
		return enrollmentReport(out, "invalid-input", 2)
	}
	ref, err := time.Parse(time.RFC3339, args[len(args)-1])
	if err != nil {
		return enrollmentReport(out, "invalid-input", 2)
	}
	var bundle enrollment.Bundle
	var candidate enrollment.Candidate
	var input release.Inputs
	var history string
	var ok bool
	initial := args[0] == "authenticate-initial"
	if initial {
		program, err := os.Executable()
		if err != nil {
			return enrollmentReport(out, protectedReleaseOutcome("unavailable", legacy), 1)
		}
		bundle, err = enrollment.Verify(ctx, enrollment.Request{BundleRoot: args[1], ExecutablePath: program, ManifestSHA256: args[2], Scope: enrollment.Headless})
		if err != nil {
			return protectedReleaseFailure(ctx, out, err, legacy)
		}
		input, ok = initialReleaseInputs(bundle, ref)
		history = args[3]
	} else {
		candidate, err = enrollment.ReadCandidate(ctx, args[1], enrollment.Headless)
		if err != nil {
			return protectedReleaseFailure(ctx, out, err, legacy)
		}
		input, ok = candidateReleaseInputs(candidate, ref)
		history = args[2]
	}
	if !ok || input.Local.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		return enrollmentReport(out, protectedReleaseOutcome("incompatible", legacy), 1)
	}
	var verifier *release.Verifier
	if initial {
		verifier, err = release.Open(history)
	} else {
		verifier, err = release.OpenRetained(history)
	}
	if err != nil {
		return protectedReleaseFailure(ctx, out, err, legacy)
	}
	var authorization generationauthorization.Authorization
	if initial {
		authorization, err = generationauthorization.AuthenticateInitial(ctx, verifier, bundle, input)
	} else {
		authorization, err = generationauthorization.AuthenticateCandidate(ctx, verifier, candidate, input)
	}
	err = errors.Join(err, verifier.Close(), ctx.Err())
	if err != nil {
		return protectedReleaseFailure(ctx, out, err, legacy)
	}
	program, generation := authorization.Targets()
	_, programOK := program.AcceptedDecision()
	_, generationOK := generation.AcceptedDecision()
	if !programOK || !generationOK {
		return enrollmentReport(out, protectedReleaseOutcome("refused", legacy), 1)
	}
	return enrollmentReport(out, "authenticated-generation", 0)
}

func protectedReleaseOutcome(suffix string, legacy bool) string {
	if legacy {
		return "installation-" + suffix
	}
	return "release-" + suffix
}

func protectedReleaseFailure(ctx context.Context, out io.Writer, err error, legacy bool) int {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
		return enrollmentReport(out, protectedReleaseOutcome("canceled", legacy), 130)
	}
	if errors.Is(err, release.ErrTrustUnavailable) {
		return enrollmentReport(out, protectedReleaseOutcome("trust-unavailable", legacy), 1)
	}
	return enrollmentReport(out, protectedReleaseOutcome("refused", legacy), 1)
}

func candidateReleaseInputs(c enrollment.Candidate, ref time.Time) (release.Inputs, bool) {
	f, ok := c.Facts()
	if !ok {
		return release.Inputs{}, false
	}
	root, rootOK := c.File(f.TrustedRoot)
	program, programOK := c.File(f.Artifact)
	if !rootOK || !programOK {
		return release.Inputs{}, false
	}
	files := make(map[string][]byte)
	for _, name := range c.MetadataNames() {
		files["https://release.invalid/metadata/"+name], _ = c.File(name)
	}
	return release.Inputs{RootBytes: root, Artifact: program, Files: files, TargetPath: f.TargetPath,
		Local: release.LocalEnvironment{Platform: f.Platform, Architecture: runtime.GOARCH,
			Environment: f.Environment, Network: f.Network, RefTime: ref.UTC()}}, true
}
