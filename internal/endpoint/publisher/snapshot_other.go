//go:build !linux && !darwin

package publisher

import (
	"context"

	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
)

// LaunchSnapshotFile freezes the selected bounded document input before launching
// its qualified worker, erasing the temporary copy after transfer.
func LaunchSnapshotFile(ctx context.Context, owner *executionruntime.Owner, principal [32]byte, _ string) (*executionruntime.Invocation, error) {
	// The native owner supplies the platform refusal before snapshot file I/O.
	return owner.LaunchPublisher(ctx, principal, nil)
}
