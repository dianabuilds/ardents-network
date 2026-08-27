//go:build !windows

package alpha

import (
	"errors"
	"os"
	"syscall"
)

func validatePersistentFloorRootPermissions(_ string, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return errors.New("alpha persistent floor root is not owned by the current user")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return errors.New("alpha persistent floor root permits group or other access")
	}
	return nil
}
