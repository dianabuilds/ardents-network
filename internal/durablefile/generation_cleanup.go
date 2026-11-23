package durablefile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// CleanupGenerations removes every content-addressed generation other than the
// retained current and previous names, then syncs the directory. Directories and
// non-generation entries are left untouched; a concurrently removed generation
// is not an error.
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
