//go:build !linux

// Unsupported native Installation operations refuse without acquiring resources.
package delivery

import (
	"context"
	"time"

	requestinput "github.com/dianabuilds/ardents-network/internal/successor/endpoint/delivery/request"
	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/successor/release/endpoint"
)

func readProvisionRequest(ctx context.Context, _ string) (requestinput.Document, error) {
	if ctx == nil {
		return requestinput.Document{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return requestinput.Document{}, err
	}
	return requestinput.Document{}, ErrNativeUnavailable
}

func provisionInitial(ctx context.Context, _ requestinput.Document, _ generationauthorization.Authorization) (ProvisionResult, error) {
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

func completeInitialRecovery(context.Context, *recoveryOperation, generationauthorization.Authorization) (ProvisionResult, error) {
	return ProvisionResult{}, ErrNativeUnavailable
}

func closeInitialRecovery(*recoveryNative) error { return nil }

type successorPreparation struct{}

func openSuccessor(context.Context, string) (*successorPreparation, requestinput.Document, error) {
	return nil, requestinput.Document{}, ErrNativeUnavailable
}

func completeSuccessor(*successorPreparation, *release.Verifier, enrollment.Candidate, release.Inputs) (ProvisionResult, error) {
	return ProvisionResult{}, ErrNativeUnavailable
}

func closeSuccessor(*successorPreparation) (error, bool) { return nil, true }

type successorRecoveryNative struct{}

func openSuccessorRecovery(context.Context, string, time.Time) (*successorRecoveryNative, requestinput.Declaration, error) {
	return nil, requestinput.Declaration{}, ErrNativeUnavailable
}

func completeSuccessorRecovery(context.Context, *successorRecoveryNative, time.Time, *release.Verifier, enrollment.Candidate, release.Inputs) (ProvisionResult, error) {
	return ProvisionResult{}, ErrNativeUnavailable
}

func closeSuccessorRecovery(*successorRecoveryNative) error { return nil }

func checkInstalled(ctx context.Context, _ string) (CheckResult, error) {
	return CheckResult{}, ErrNativeUnavailable
}
