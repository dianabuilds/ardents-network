//go:build linux

package main

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
)

func launchSelectedPublisher(ctx context.Context, owner *executionruntime.Owner, principal [32]byte, path string) (*executionruntime.Invocation, error) {
	body, err := textdocument.ReadSnapshotFile(ctx, path)
	if err != nil {
		return nil, err
	}
	defer clear(body)
	return owner.LaunchPublisher(ctx, principal, body)
}
