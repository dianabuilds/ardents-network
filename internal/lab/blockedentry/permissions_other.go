//go:build !windows

package blockedentry

import (
	"errors"
	"io/fs"
	"path/filepath"
)

func protectEvidenceTree(root string) error {
	return filepath.WalkDir(root, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.Type()&fs.ModeSymlink != 0 {
			return errors.Join(walkErr, errors.New("evidence tree is unavailable or aliased"))
		}
		info, err := entry.Info()
		if err != nil || info.Mode().Perm()&0o077 != 0 {
			return errors.Join(err, errors.New("evidence tree permits group or other access"))
		}
		return nil
	})
}
