//go:build !linux

package main

import (
	"context"

	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
)

func launchSelectedPublisher(ctx context.Context, owner *executionruntime.Owner, principal [32]byte, _ string) (*executionruntime.Invocation, error) {
	// The native owner supplies the platform refusal before snapshot file I/O.
	return owner.LaunchPublisher(ctx, principal, nil)
}
