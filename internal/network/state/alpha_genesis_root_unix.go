//go:build !windows

package state

import (
	"errors"
	"os"
	"syscall"
)

func validateAlphaGenesisRootAccess(_ string, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0o077 != 0 {
		return errors.New("functional alpha State root is not owner-only")
	}
	return nil
}
