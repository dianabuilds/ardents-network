//go:build !windows

package endpoint

import (
	"errors"
	"os"
	"syscall"
)

func secureTransitAcquisitionRoot(root string, _ os.FileInfo) error {
	if err := os.Chmod(root, 0o700); err != nil {
		return err
	}
	var status syscall.Stat_t
	if err := syscall.Lstat(root, &status); err != nil {
		return err
	}
	if status.Mode&syscall.S_IFMT != syscall.S_IFDIR || status.Mode&0o777 != 0o700 || status.Uid != uint32(os.Geteuid()) {
		return errors.New("transit acquisition root is not private to the Endpoint user")
	}
	return nil
}
