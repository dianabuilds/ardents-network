package main

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"time"

	installation "github.com/dianabuilds/ardents-network/internal/successor/endpoint/delivery"
	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/successor/release/endpoint"
)

// Transitional native delivery commands retain their own original custody.
func runInstallation(ctx context.Context, args []string, out io.Writer) int {
	if len(args) == 3 && args[0] == "recover-installed" {
		reference, err := time.Parse(time.RFC3339Nano, args[2])
		if err != nil || reference.UTC().Format(time.RFC3339Nano) != args[2] {
			return enrollmentReport(out, "invalid-input", 2)
		}
		return runSuccessorRecovery(ctx, args[1], reference, out)
	}
	if len(args) == 3 && args[0] == "upgrade-installed" && args[1] == "--request" {
		return runInstallationSuccessor(ctx, args[2], out)
	}
	if len(args) == 3 && args[0] == "recover-initial" {
		reference, err := time.Parse(time.RFC3339Nano, args[2])
		if err != nil || reference.UTC().Format(time.RFC3339Nano) != args[2] {
			return enrollmentReport(out, "invalid-input", 2)
		}
		return runInitialRecovery(ctx, args[1], reference, out)
	}
	if len(args) == 2 && args[0] == "check" {
		result, err := installation.Check(ctx, args[1])
		if err != nil {
			return installationFailure(ctx, out, err)
		}
		return enrollmentReport(out, result.Status, 0)
	}
	if len(args) == 3 && args[0] == "provision" && args[1] == "--request" {
		return runInstallationProvision(ctx, args[2], out)
	}
	return enrollmentReport(out, "invalid-input", 2)
}

func runInstallationSuccessor(ctx context.Context, filename string, out io.Writer) int {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	owner, err := installation.OpenSuccessor(ctx, filename)
	if err != nil {
		return installationFailure(ctx, out, err)
	}
	request := owner.Request()
	candidate, err := enrollment.ReadCandidate(ctx, request.BundleRoot(), enrollment.Headless)
	if err != nil {
		return installationFailure(ctx, out, errors.Join(err, owner.Close()))
	}
	input, ok := candidateReleaseInputs(candidate, request.ReferenceTime())
	if !ok || input.Local.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		return installationFailure(ctx, out, errors.Join(installation.ErrBinding, owner.Close()))
	}
	verifier, err := release.OpenRetained(request.ReleaseHistoryRoot())
	if err != nil {
		return installationFailure(ctx, out, errors.Join(err, owner.Close()))
	}
	result, err := owner.Complete(verifier, candidate, input)
	err = errors.Join(err, owner.Close(), verifier.Close(), ctx.Err())
	if result.Status != "" && err != nil {
		// Release-close, output and original cancellation cannot revoke a full ACK.
		return enrollmentReport(out, "installed-started-recovery-required", 1)
	}
	if err != nil {
		return installationFailure(ctx, out, err)
	}
	return enrollmentReport(out, result.Status, 0)
}

func runInitialRecovery(ctx context.Context, root string, reference time.Time, out io.Writer) int {
	owner, err := installation.OpenInitialRecovery(ctx, root, reference)
	if err != nil {
		return installationFailure(ctx, out, err)
	}
	candidate, err := enrollment.ReadCandidate(ctx, owner.BundleRoot(), enrollment.Headless)
	if err != nil {
		return installationFailure(ctx, out, errors.Join(err, owner.Close()))
	}
	input, ok := candidateReleaseInputs(candidate, owner.ReferenceTime())
	if !ok || input.Local.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		return installationFailure(ctx, out, errors.Join(installation.ErrBinding, owner.Close()))
	}
	verifier, err := release.OpenRetained(owner.ReleaseHistoryRoot())
	if err != nil {
		return installationFailure(ctx, out, errors.Join(err, owner.Close()))
	}
	authorization, err := generationauthorization.AuthenticateCandidate(ctx, verifier, candidate, input)
	var result installation.ProvisionResult
	if err == nil {
		result, err = owner.Complete(authorization)
	}
	// Retain the independent Release lease until Installation's original physical
	// borrowers have closed. No initial-pin bootstrap or floor reset is possible.
	err = errors.Join(err, owner.Close(), verifier.Close(), ctx.Err())
	if err != nil {
		return installationFailure(ctx, out, err)
	}
	return enrollmentReport(out, result.Status, 0)
}

func runSuccessorRecovery(ctx context.Context, root string, reference time.Time, out io.Writer) int {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	owner, err := installation.OpenSuccessorRecovery(ctx, root, reference)
	if err != nil {
		return installationFailure(ctx, out, err)
	}
	candidate, err := enrollment.ReadCandidate(ctx, owner.BundleRoot(), enrollment.Headless)
	if err != nil {
		return installationFailure(ctx, out, errors.Join(err, owner.Close()))
	}
	input, ok := candidateReleaseInputs(candidate, owner.ReferenceTime())
	if !ok || input.Local.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		return installationFailure(ctx, out, errors.Join(installation.ErrBinding, owner.Close()))
	}
	verifier, err := release.OpenRetained(owner.ReleaseHistoryRoot())
	if err != nil {
		return installationFailure(ctx, out, errors.Join(err, owner.Close()))
	}
	result, err := owner.Complete(verifier, candidate, input)
	err = errors.Join(err, owner.Close(), verifier.Close(), ctx.Err())
	if err != nil {
		if result.Status != "" {
			return enrollmentReport(out, "installed-started-recovery-required", 1)
		}
		return installationFailure(ctx, out, err)
	}
	return enrollmentReport(out, result.Status, 0)
}

func runInstallationProvision(ctx context.Context, filename string, out io.Writer) int {
	request, err := installation.ReadProvisionRequest(ctx, filename)
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
	verifier, err := release.Open(request.ReleaseHistoryRoot())
	if err != nil {
		return installationFailure(ctx, out, err)
	}
	authorization, err := generationauthorization.AuthenticateInitial(ctx, verifier, bundle, input)
	var result installation.ProvisionResult
	if err == nil {
		result, err = installation.ProvisionInitial(ctx, request, authorization)
	}
	// The separate Release history lease outlives Installation's physical
	// borrowers. Late refusal preserves floors and cannot publish success.
	err = errors.Join(err, verifier.Close(), ctx.Err())
	if err != nil {
		return installationFailure(ctx, out, err)
	}
	return enrollmentReport(out, result.Status, 0)
}

func installationFailure(ctx context.Context, out io.Writer, err error) int {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
		return enrollmentReport(out, "installation-canceled", 130)
	}
	if errors.Is(err, release.ErrTrustUnavailable) {
		return enrollmentReport(out, "installation-trust-unavailable", 1)
	}
	if errors.Is(err, installation.ErrRepairRequired) {
		return enrollmentReport(out, "installation-repair-required", 1)
	}
	return enrollmentReport(out, "installation-refused", 1)
}
