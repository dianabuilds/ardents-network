package installation

import (
	"context"
	"errors"
	"time"
)

// Upgrade replaces one established installation with a freshly authenticated
// strictly newer generation. Its receipt observes the fixed unit, not Service
// publication continuity or complete installed qualification.
func Upgrade(ctx context.Context, path string) (ProvisionResult, error) {
	if ctx == nil {
		return ProvisionResult{}, errors.New("installation upgrade context is unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := observePlatform(bounded); err != nil {
		return ProvisionResult{}, err
	}
	return upgradeInstalled(bounded, path)
}
