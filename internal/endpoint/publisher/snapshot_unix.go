//go:build linux || darwin

package publisher

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/application/document"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
)

// LaunchSnapshotFile freezes the selected bounded document input before launching
// its qualified worker, erasing the temporary copy after transfer. The input
// import (document.ReadSnapshotFile) only reads the caller's explicitly selected
// file read-only and creates no filesystem state or authority, so on macOS the
// native owner's typed launch refusal still lands before any state or worker
// (#883); reading caller input is not state creation and no unconfined worker is
// spawned.
func LaunchSnapshotFile(ctx context.Context, owner *executionruntime.Owner, principal [32]byte, path string) (*executionruntime.Invocation, error) {
	body, err := document.ReadSnapshotFile(ctx, path)
	if err != nil {
		return nil, err
	}
	defer clear(body)
	return owner.LaunchPublisher(ctx, principal, body)
}
