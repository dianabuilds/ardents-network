//go:build !linux

package installation

import (
	"context"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

type successorRecoveryNative struct{}

func openSuccessorRecovery(context.Context, string, time.Time) (*successorRecoveryNative, installationRequest, error) {
	return nil, installationRequest{}, ErrNativeUnavailable
}
func completeSuccessorRecovery(context.Context, *successorRecoveryNative, time.Time, *release.Verifier, enrollment.Candidate, release.Inputs) (ProvisionResult, error) {
	return ProvisionResult{}, ErrNativeUnavailable
}
func closeSuccessorRecovery(*successorRecoveryNative) error { return nil }
