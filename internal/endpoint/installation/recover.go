package installation

import (
	"context"
	"errors"
	"time"
)

// Recover explicitly resumes an owned successor intent after fresh Release
// authentication. It never lowers floors or treats retained bytes as authority.
func Recover(ctx context.Context, root string) (ProvisionResult, error) {
	if ctx == nil {
		return ProvisionResult{}, errors.New("installation recovery context is unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := observePlatform(bounded); err != nil {
		return ProvisionResult{}, err
	}
	return recoverInstalled(bounded, root)
}
