//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/successor/installation"
	installation "github.com/dianabuilds/ardents-network/internal/successor/installation/endpoint"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

func runPlatformStageEndpoint(ctx context.Context, filename string, out io.Writer) int {
	request, err := installation.ReadOwnedRequest(ctx, filename, true)
	if err != nil {
		return installationFailure(ctx, out, err)
	}
	program, err := os.Executable()
	if err != nil {
		return installationFailure(ctx, out, err)
	}
	bundle, err := enrollment.Verify(ctx, enrollment.Request{BundleRoot: request.BundleRoot(), ExecutablePath: program,
		ManifestSHA256: request.ManifestSHA256(), Scope: enrollment.Headless})
	if err != nil {
		return installationFailure(ctx, out, err)
	}
	input, ok := initialReleaseInputs(bundle, request.ReferenceTime())
	if !ok || input.Local.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		return enrollmentReport(out, "installation-incompatible", 1)
	}
	verifier, err := release.OpenRetained(request.ReleaseHistoryRoot())
	if err != nil {
		return installationFailure(ctx, out, err)
	}
	// Staging may use only the exact metadata already committed by the
	// first-pin step. It cannot advance trust before a later staging refusal.
	if err := verifyStageReleaseMetadata(ctx, verifier, input); err != nil {
		return installationFailure(ctx, out, errors.Join(err, verifier.Close()))
	}
	authorization, err := generationauthorization.AuthenticateInitial(ctx, verifier, bundle, input)
	if err == nil {
		err = installation.StagePlatformGeneration(ctx, request, authorization, verifier)
	}
	if err = errors.Join(err, verifier.Close(), ctx.Err()); err != nil {
		return installationFailure(ctx, out, err)
	}
	return enrollmentReport(out, "generation-staged", 0)
}

func runPlatformStageEndpointSuccessor(ctx context.Context, filename, previousManifest string, out io.Writer) int {
	if err := stagePlatformEndpointSuccessor(ctx, filename, previousManifest); err != nil {
		return installationFailure(ctx, out, err)
	}
	return enrollmentReport(out, "generation-staged", 0)
}

func stagePlatformEndpointSuccessor(ctx context.Context, filename, previousManifest string) (returnedErr error) {
	request, err := installation.ReadOwnedRequest(ctx, filename, false)
	if err != nil {
		return err
	}
	if !platformBelow("/etc/ardents", previousManifest) {
		return errPlatformInput
	}
	files := &platformFiles{}
	defer func() { returnedErr = errors.Join(returnedErr, files.Close()) }()
	raw, err := files.Read(previousManifest, 16<<10, false)
	if err != nil {
		return err
	}
	var previous platformLaunch
	if decodeAdmissionObject(raw, &previous) != nil || previous.Role != "endpoint" ||
		!validEndpointPlatformLaunch(previous) || previous.ReleaseRoot != request.ReleaseHistoryRoot() {
		return errPlatformInput
	}
	body, err := files.Read(previous.Configuration, 64<<10, false)
	if err != nil || platformDigest(body) != previous.ConfigSHA256 {
		return errors.Join(errPlatformInput, err)
	}
	decl, err := platformEndpointDeclaration(body)
	if err != nil || decl.ReleaseFloorRoot != previous.ReleaseRoot {
		return errors.Join(errPlatformInput, err)
	}
	candidate, err := enrollment.ReadCandidate(ctx, request.BundleRoot(), enrollment.Headless)
	if err != nil {
		return err
	}
	input, ok := candidateReleaseInputs(candidate, request.ReferenceTime())
	if !ok || input.Local.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		return errPlatformInput
	}
	if err := files.Check(ctx); err != nil {
		return err
	}
	verifier, err := release.OpenRetained(request.ReleaseHistoryRoot())
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, verifier.Close(), ctx.Err()) }()
	return installation.StagePlatformSuccessor(ctx, request, candidate, input, verifier, decl.InstallationRoot,
		filepath.Join(previous.Distribution, "ardents-linux-amd64"), previousManifest, platformDigest(raw))
}

func verifyStageReleaseMetadata(ctx context.Context, verifier *release.Verifier, input release.Inputs) error {
	floors, err := verifier.CurrentFloors(ctx)
	if err != nil || !floors.Complete() {
		return errors.Join(release.ErrTrustUnavailable, err)
	}
	const metadata = "https://release.invalid/metadata/"
	versioned := func(version int64, role string) []byte {
		return input.Files[metadata+strconv.FormatInt(version, 10)+"."+role+".json"]
	}
	for _, pair := range []struct{ body, digest []byte }{
		{input.RootBytes, floors.RootDigest},
		{input.Files[metadata+"timestamp.json"], floors.TimestampDigest},
		{versioned(floors.SnapshotVersion, "snapshot"), floors.SnapshotDigest},
		{versioned(floors.TargetsVersion, "targets"), floors.TargetsDigest},
	} {
		sum := sha256.Sum256(pair.body)
		if len(pair.body) == 0 || !bytes.Equal(sum[:], pair.digest) {
			return release.ErrTrustUnavailable
		}
	}
	return ctx.Err()
}
