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
		return errors.New("browser enrollment entry is not an owner-only single-link regular file")
	}
	return nil
}
