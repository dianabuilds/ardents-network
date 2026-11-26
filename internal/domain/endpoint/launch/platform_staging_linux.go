package launch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"

	requestinput "github.com/dianabuilds/ardents-network/internal/domain/endpoint/plan"
	"github.com/dianabuilds/ardents-network/internal/domain/enrollment"
	"github.com/dianabuilds/ardents-network/internal/domain/release"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/domain/release/endpoint"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// ErrPlatformIncompatible indicates that the initial distribution targets a
// different platform or cannot supply its own Release inputs.
var ErrPlatformIncompatible = errors.New("incompatible Endpoint distribution")

// StagePlatformInitial authenticates and seals the first Endpoint generation
// against the exact metadata already committed by the initial trust operation.
func StagePlatformInitial(ctx context.Context, filename string) error {
	request, err := ReadOwnedRequest(ctx, filename, true)
	if err != nil {
		return err
	}
	program, err := os.Executable()
	if err != nil {
		return err
	}
	bundle, err := enrollment.Verify(ctx, enrollment.Request{BundleRoot: request.BundleRoot(), ExecutablePath: program,
		ManifestSHA256: request.ManifestSHA256()})
	if err != nil {
		return err
	}
	input, ok := generationauthorization.InitialInputs(bundle, request.ReferenceTime())
	if !ok || input.Local.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		return ErrPlatformIncompatible
	}
	verifier, err := release.OpenRetained(request.ReleaseHistoryRoot())
	if err != nil {
		return err
	}
	// Staging may use only the exact metadata already committed by the
	// first-pin step. It cannot advance trust before a later staging refusal.
	if err := generationauthorization.VerifyStagingMetadata(ctx, verifier, input); err != nil {
		return errors.Join(err, verifier.Close())
	}
	authorization, err := generationauthorization.AuthenticateInitial(ctx, verifier, bundle, input)
	if err == nil {
		err = StagePlatformGeneration(ctx, request, authorization, verifier)
	}
	if err = errors.Join(err, verifier.Close(), ctx.Err()); err != nil {
		return err
	}
	return nil
}

// StagePlatformReplacement seals the authorized successor after observing the
// exact stopped original service and retaining the previous configuration.
func StagePlatformReplacement(ctx context.Context, filename, previousManifest string) (returnedErr error) {
	request, err := ReadOwnedRequest(ctx, filename, false)
	if err != nil {
		return err
	}
	if !requestinput.PlatformBelow("/etc/ardents", previousManifest) {
		return errPlatformInput
	}
	files := &artifacts.PlatformInputs{}
	defer func() { returnedErr = errors.Join(returnedErr, files.Close()) }()
	raw, err := files.Read(previousManifest, 16<<10, false)
	if err != nil {
		return err
	}
	var previous requestinput.PlatformLaunch
	if artifacts.DecodeObject(raw, &previous) != nil || !previous.Valid() || previous.ReleaseRoot != request.ReleaseHistoryRoot() {
		return errPlatformInput
	}
	body, err := files.Read(previous.Configuration, 64<<10, false)
	if err != nil || digestHex(body) != previous.ConfigSHA256 {
		return errors.Join(errPlatformInput, err)
	}
	decl, err := requestinput.DecodeDeclaration(body)
	if err != nil || decl.ReleaseFloorRoot != previous.ReleaseRoot {
		return errors.Join(errPlatformInput, err)
	}
	candidate, err := enrollment.ReadCandidate(ctx, request.BundleRoot())
	if err != nil {
		return err
	}
	input, ok := generationauthorization.CandidateInputs(candidate, request.ReferenceTime())
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
	return StagePlatformSuccessor(ctx, request, candidate, input, verifier, decl.InstallationRoot,
		filepath.Join(previous.Distribution, "ardents-linux-amd64"), previousManifest, digestHex(raw))
}
