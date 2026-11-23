//go:build linux

package main

import (
	"context"
	"errors"
	"io"

	endpointlaunch "github.com/dianabuilds/ardents-network/internal/endpoint/launch"
	"github.com/dianabuilds/ardents-network/internal/release"
)

func runPlatformStageEndpoint(ctx context.Context, filename string, out io.Writer) int {
	err := endpointlaunch.StagePlatformInitial(ctx, filename)
	if errors.Is(err, endpointlaunch.ErrPlatformIncompatible) {
		return enrollmentReport(out, "installation-incompatible", 1)
	}
	if err != nil {
		return reportSoftwareDeliveryFailure(ctx, out, err)
	}
	return enrollmentReport(out, "generation-staged", 0)
}

func runPlatformStageEndpointSuccessor(ctx context.Context, filename, previousManifest string, out io.Writer) int {
	if err := endpointlaunch.StagePlatformReplacement(ctx, filename, previousManifest); err != nil {
		return reportSoftwareDeliveryFailure(ctx, out, err)
	}
	return enrollmentReport(out, "generation-staged", 0)
}

func reportSoftwareDeliveryFailure(ctx context.Context, out io.Writer, err error) int {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
		return enrollmentReport(out, "installation-canceled", 130)
	}
	if errors.Is(err, release.ErrTrustUnavailable) {
		return enrollmentReport(out, "installation-trust-unavailable", 1)
	}
	return enrollmentReport(out, "installation-refused", 1)
}
