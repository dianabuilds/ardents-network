//go:build !windows

package instance

import (
	"os"
	"syscall"
)

func validateRootAccess(_ string, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0o077 != 0 {
		return ErrInvalid
	}
	return nil
}
