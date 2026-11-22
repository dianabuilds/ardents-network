//go:build linux

package publisher

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/application/document"
	executionruntime "github.com/dianabuilds/ardents-network/internal/execution/runtime"
)

// LaunchSnapshotFile freezes the selected bounded Text input before launching
// its qualified worker, erasing the temporary copy after transfer.
func LaunchSnapshotFile(ctx context.Context, owner *executionruntime.Owner, principal [32]byte, path string) (*executionruntime.Invocation, error) {
	body, err := document.ReadSnapshotFile(ctx, path)
	if err != nil {
		return nil, err
	}
	defer clear(body)
	return owner.LaunchPublisher(ctx, principal, body)
}
