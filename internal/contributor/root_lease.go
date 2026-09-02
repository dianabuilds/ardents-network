package contributor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var errContributorRootBusy = errors.New("contributor root is busy")

func (profile *Profile) acquireRootLease() (rootLease, error) {
	if err := os.MkdirAll(filepath.Dir(profile.paths.lease), 0o755); err != nil {
		return rootLease{}, fmt.Errorf("create Contributor root-lease directory: %w", err)
	}
	return acquireRootLeaseFile(profile.paths.lease)
}
