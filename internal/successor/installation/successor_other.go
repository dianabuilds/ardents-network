//go:build !linux

package installation

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

type successorPreparation struct{}

func openSuccessor(context.Context, string) (*successorPreparation, Request, error) {
	return nil, Request{}, ErrNativeUnavailable
}

func completeSuccessor(*successorPreparation, *release.Verifier, enrollment.Candidate, release.Inputs) (ProvisionResult, error) {
	return ProvisionResult{}, ErrNativeUnavailable
}

func closeSuccessor(*successorPreparation) (error, bool) { return nil, true }
