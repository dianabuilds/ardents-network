//go:build linux

package blockedverify

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

func protectRegistryTree(root string) error {
	expectedUID := uint32(os.Geteuid())
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.Type()&fs.ModeSymlink != 0 {
			return errors.Join(walkErr, errors.New("replay registry is unavailable or aliased"))
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		stat, owned := info.Sys().(*syscall.Stat_t)
		if !owned || stat.Uid != expectedUID {
			return errors.New("replay registry has a foreign owner")
		}
		mode := fs.FileMode(0o600)
		if entry.IsDir() {
			mode = 0o700
		}
		if err := os.Chmod(path, mode); err != nil {
			return err
		}
		info, err = os.Lstat(path)
		if err != nil || info.Mode().Perm()&0o077 != 0 {
			return errors.Join(err, errors.New("replay registry is not owner-only"))
		}
		return nil
	})
}
