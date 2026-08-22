package release

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// cleanupFloorStoreStaging removes interrupted staging from a previous
// crash. The scan is bounded so a corrupt state root fails closed.
func cleanupFloorStoreStaging(root, generations string) error {
	for directory, prefix := range map[string]string{root: ".current-", generations: ".stage-"} {
		entries, err := readFloorStoreDirectory(directory, 64)
		if err != nil {
			return fmt.Errorf("release: scan state root: %w", err)
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), prefix) {
				continue
			}
			if err := os.RemoveAll(filepath.Join(directory, entry.Name())); err != nil {
				return fmt.Errorf("release: remove interrupted staging %q: %w", entry.Name(), err)
			}
		}
	}
	return nil
}
