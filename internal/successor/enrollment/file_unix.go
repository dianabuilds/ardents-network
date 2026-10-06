//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package enrollment

import (
	"os"
	"syscall"
)

// Native ownership and no-follow/nonblocking open apply to Unix filesystem
// objects, not the portable manifest/descriptor rules.
func verifyOwnedFile(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) || stat.Nlink != 1 || info.Mode().Perm()&0o077 != 0 {
		return ErrInventory
	}
	return nil
}

func openBundleFile(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}
