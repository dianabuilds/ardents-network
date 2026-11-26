package durablefile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// CleanupGenerations removes non-directory entries in the caller-owned state-
// namespace, except the current and previous names, then syncs the directory.
// The suffix need not be a canonical generation digest. Entries outside that
// namespace and directories are left untouched; a concurrently removed entry
// is not an error. The caller must supply an exclusively owned root.
func CleanupGenerations(root, current, previous string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "state-") {
			continue
		}
		name := strings.TrimPrefix(entry.Name(), "state-")
		if name != current && name != previous {
			if err := os.Remove(filepath.Join(root, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return SyncDirectory(root)
}
