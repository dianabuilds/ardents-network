package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// These commands authenticate generation bytes only. They do not provision,
// change installed selection, start a worker or establish Service readiness.
func runInstallation(ctx context.Context, args []string, out io.Writer) int {
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
			return enrollmentReport(out, "installation-unavailable", 1)
		}
		bundle, err = enrollment.Verify(ctx, enrollment.Request{BundleRoot: args[1], ExecutablePath: program, ManifestSHA256: args[2], Scope: enrollment.Headless})
		if err != nil {
			return installationFailure(ctx, out, err)
		}
		input, ok = initialReleaseInputs(bundle, ref)
		history = args[3]
	} else {
		candidate, err = enrollment.ReadCandidate(ctx, args[1], enrollment.Headless)
		if err != nil {
			return installationFailure(ctx, out, err)
		}
		input, ok = candidateReleaseInputs(candidate, ref)
		history = args[2]
	}
	if !ok || input.Local.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		return enrollmentReport(out, "installation-incompatible", 1)
	}
	var verifier *release.Verifier
	if initial {
		verifier, err = release.Open(history)
	} else {
		verifier, err = release.OpenRetained(history)
	}
	if err != nil {
		return installationFailure(ctx, out, err)
	}
	var authorization installation.Authorization
	if initial {
		authorization, err = installation.AuthenticateInitial(ctx, verifier, bundle, input)
	} else {
		authorization, err = installation.AuthenticateCandidate(ctx, verifier, candidate, input)
	}
	err = errors.Join(err, verifier.Close(), ctx.Err())
	if err != nil {
		return installationFailure(ctx, out, err)
	}
	program, generation := authorization.Targets()
	p, programOK := program.AcceptedDecision()
	g, generationOK := generation.AcceptedDecision()
	if !programOK || !generationOK {
		return enrollmentReport(out, "installation-refused", 1)
	}
	actualProgram, ok := authorization.Resource("ardents-linux-amd64")
	programDigest := sha256.Sum256(actualProgram)
	generationDigest := sha256.Sum256(authorization.Descriptor())
	if !ok || !bytes.Equal(p.Digest, programDigest[:]) || !bytes.Equal(g.Digest, generationDigest[:]) {
		return enrollmentReport(out, "installation-refused", 1)
	}
	return enrollmentReport(out, "authenticated-generation", 0)
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

func installationFailure(ctx context.Context, out io.Writer, err error) int {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
		return enrollmentReport(out, "installation-canceled", 130)
	}
	if errors.Is(err, release.ErrTrustUnavailable) {
		return enrollmentReport(out, "installation-trust-unavailable", 1)
	}
	return enrollmentReport(out, "installation-refused", 1)
}
