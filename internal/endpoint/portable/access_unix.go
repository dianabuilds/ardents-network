//go:build !windows

package portable

import (
	"errors"
	"os"
	"syscall"
)

func validateRuntimeBase(path string) error { return validateDirectory(path, false) }

func secureOwnedDirectory(path string, _ os.FileInfo) error { return validateDirectory(path, true) }

func validateDirectory(path string, enforceMode bool) error {
	if enforceMode {
		if err := os.Chmod(path, 0o700); err != nil {
			return err
		}
	}
	var status syscall.Stat_t
	if err := syscall.Lstat(path, &status); err != nil {
		return err
	}
	if status.Mode&syscall.S_IFMT != syscall.S_IFDIR || status.Uid != uint32(os.Geteuid()) ||
		(enforceMode && status.Mode&0o777 != 0o700) || (!enforceMode && status.Mode&0o077 != 0) {
		return errors.New("local runtime directory is not private to the endpoint user")
	}
	return nil
}

func secureAttachment(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	var status syscall.Stat_t
	if err := syscall.Lstat(path, &status); err != nil {
		return err
	}
	if status.Mode&syscall.S_IFMT != syscall.S_IFSOCK || status.Mode&0o777 != 0o600 || status.Uid != uint32(os.Geteuid()) {
		return errors.New("local attachment is not a private socket")
	}
	return nil
}
