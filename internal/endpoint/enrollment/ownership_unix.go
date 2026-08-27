//go:build !windows

package enrollment

import (
	"errors"
	"os"
	"syscall"
)

func verifyOwnedRegular(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) || stat.Nlink != 1 || info.Mode().Perm()&0o077 != 0 {
		return errors.New("entry is not an owner-only single-link regular file")
	}
	return nil
}

func verifyPackageFile(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 || stat.Nlink != 1 || info.Mode().Perm()&0o022 != 0 {
		return errors.New("package file is not a root-owned non-writable single-link regular file")
	}
	return nil
}

func verifyPackageDirectory(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 || info.Mode().Perm()&0o022 != 0 {
		return errors.New("package static root is not a root-owned non-writable directory")
	}
	return nil
}
