package contributor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var errContributorRootBusy = errors.New("contributor root is busy")

func (profile *Profile) acquireRootLease() (rootLease, error) {
	directory := filepath.Dir(profile.paths.privateRoot)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return rootLease{}, fmt.Errorf("create Contributor root-lease directory: %w", err)
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return rootLease{}, fmt.Errorf("inspect Contributor root-lease directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return rootLease{}, errors.New("contributor root-lease directory is invalid")
	}
	return acquireRootLeaseDirectory(directory)
}

func wrapRootLeaseRelease(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s Contributor root lease: %w", action, err)
}
