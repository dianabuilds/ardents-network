//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package blockedentry

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

func validateFinalConfigurationTree(root string) error {
	return filepath.WalkDir(root, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.Type()&fs.ModeSymlink != 0 {
			return errors.Join(walkErr, errors.New("final configuration tree is unavailable or aliased"))
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0o077 != 0 {
			return errors.New("final configuration tree is not owner-only")
		}
		if entry.IsDir() {
			if info.Mode().Perm()&0o200 != 0 || info.Mode().Perm()&0o500 != 0o500 {
				return errors.New("final configuration directory is mutable or unreadable")
			}
			return nil
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0o222 != 0 {
			return errors.New("final configuration file is mutable or not regular")
		}
		return nil
	})
}
