package endpoint

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
)

// InitialVerification reports the joined result of verifying the running
// Endpoint against an independently pinned bundle and retained Release history.
type InitialVerification struct {
	Outcome     string
	RootVersion int64
	Detailed    bool
	Authorized  bool
	Canceled    bool
}

// VerifyInitial verifies the running program and joins the Release verifier
// before returning any result. It does not install or start an Endpoint.
func VerifyInitial(ctx context.Context, bundleRoot, pin, history string, ref time.Time) InitialVerification {
	unavailable := InitialVerification{Outcome: "release-unavailable"}
	if ctx == nil {
		return unavailable
	}
	program, err := os.Executable()
	if err != nil {
		return unavailable
	}
	bundle, err := enrollment.Verify(ctx, enrollment.Request{BundleRoot: bundleRoot, ExecutablePath: program, ManifestSHA256: pin})
	if err != nil {
		return InitialVerification{Outcome: "enrollment-refused"}
	}
	if _, ok := bundle.CurrentPlatformFacts(); !ok {
		return InitialVerification{Outcome: "release-incompatible"}
	}
	inputs, ok := InitialInputs(bundle, ref)
	if !ok {
		return InitialVerification{Outcome: "release-invalid"}
	}
	verifier, err := release.Open(history)
	if err != nil {
		return unavailable
	}
	decision := verifier.Evaluate(ctx, inputs)
	var handoffErr error
	if proof, ok := decision.Authorization(); ok {
		handoffErr = verifier.CheckAuthorizations(ctx, proof)
	}
	floors, floorErr := verifier.CurrentFloors(ctx)
	floorErr = errors.Join(floorErr, handoffErr)
	closeErr := verifier.Close()
	if closeErr == nil && (errors.Is(decision.Err(), context.Canceled) || errors.Is(decision.Err(), context.DeadlineExceeded)) {
		unavailable.Canceled = true
		return unavailable
	}
	if closeErr != nil || floorErr != nil || ctx.Err() != nil {
		return unavailable
	}
	result := InitialVerification{Detailed: true, RootVersion: floors.RootVersion}
	if authorization, ok := decision.Authorization(); ok {
		accepted, valid := authorization.AcceptedDecision()
		if !valid {
			return InitialVerification{Outcome: "release-invalid"}
		}
		decision = accepted
		result.Authorized = true
	}
	result.Outcome = string(decision.Outcome)
	return result
}
