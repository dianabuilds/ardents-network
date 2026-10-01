package installation

import (
	"context"
	"errors"
	"time"
)

// ProvisionResult observes one selected stopped installation. It grants no
// participant readiness, permission or restart/recovery authorization.
type ProvisionResult struct {
	Status           string `json:"status"`
	GenerationDigest string `json:"generation_digest"`
	Role             string `json:"role"`
}

// Provision installs the independently pinned initial generation without start.
// Failed transitions and advanced Release floors are retained for explicit repair.
func Provision(ctx context.Context, path string) (ProvisionResult, error) {
	if ctx == nil {
		return ProvisionResult{}, errors.New("installation provisioning context is unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := observePlatform(bounded); err != nil {
		return ProvisionResult{}, err
	}
	return provisionInitial(bounded, path)
}
