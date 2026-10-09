//go:build !linux

// Unsupported native Installation operations refuse without acquiring resources.
package installation

import (
	"context"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

func readProvisionRequest(ctx context.Context, _ string) (Request, error) {
	if ctx == nil {
		return Request{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return Request{}, err
	}
	return Request{}, ErrNativeUnavailable
}

func provisionInitial(ctx context.Context, _ Request, _ Authorization) (ProvisionResult, error) {
	if ctx == nil {
		return ProvisionResult{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return ProvisionResult{}, err
	}
	return ProvisionResult{}, ErrNativeUnavailable
}

type recoveryNative struct{}

func openInitialRecovery(context.Context, string, time.Time) (*Recovery, error) {
	return nil, ErrNativeUnavailable
}

func completeInitialRecovery(context.Context, *recoveryOperation, Authorization) (ProvisionResult, error) {
	return ProvisionResult{}, ErrNativeUnavailable
}

func closeInitialRecovery(*recoveryNative) error { return nil }

type successorPreparation struct{}

func openSuccessor(context.Context, string) (*successorPreparation, Request, error) {
	return nil, Request{}, ErrNativeUnavailable
}

func completeSuccessor(*successorPreparation, *release.Verifier, enrollment.Candidate, release.Inputs) (ProvisionResult, error) {
	return ProvisionResult{}, ErrNativeUnavailable
}

func closeSuccessor(*successorPreparation) (error, bool) { return nil, true }

type successorRecoveryNative struct{}

func openSuccessorRecovery(context.Context, string, time.Time) (*successorRecoveryNative, installationRequest, error) {
	return nil, installationRequest{}, ErrNativeUnavailable
}

func completeSuccessorRecovery(context.Context, *successorRecoveryNative, time.Time, *release.Verifier, enrollment.Candidate, release.Inputs) (ProvisionResult, error) {
	return ProvisionResult{}, ErrNativeUnavailable
}

func closeSuccessorRecovery(*successorRecoveryNative) error { return nil }

func checkInstalled(ctx context.Context, _ string) (CheckResult, error) {
	return CheckResult{}, ErrNativeUnavailable
}
