package descriptor

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/nodeprincipal"
)

// observeNodePrincipal keeps the role's error vocabulary while the observer
// independently owns and closes each original host-file and NSS acquisition.
func observeNodePrincipal(ctx context.Context, uid, gid uint32) error {
	return nodeprincipal.MapObservationError(nodeprincipal.Observe(ctx, uid, gid), ErrInput, ErrBinding)
}
