package custody

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/platform/ext4"
)

const tmpfsFilesystem = 0x01021994

func checkPrivatePermissionDestination(path string) error {
	destination, err := filepath.Abs(path)
	if err != nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	parent, err := os.Open(filepath.Dir(destination))
	if err != nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	info, statErr := parent.Stat()
	if statErr != nil || !info.IsDir() {
		return errors.Join(ErrPrivatePermissionUnavailable, statErr, parent.Close())
	}
	return errors.Join(checkPrivatePermissionFilesystem(parent), parent.Close())
}

func checkPrivatePermissionFilesystem(file *os.File) error {
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(file.Fd()), &fs); err != nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	if fs.Type == tmpfsFilesystem {
		return nil
	}
	if err := ext4.Require(file); err != nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	return nil
}

func privatePermissionFile(info os.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid())
}
