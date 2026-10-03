//go:build linux

package quota

import (
	"errors"
	"os"
	"syscall"
)

func admissionPlatform() error { return nil }
func privateAdmissionFile(info os.FileInfo, directory bool) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	if directory {
		return info.IsDir() && info.Mode().Perm() == 0700
	}
	return info.Mode().IsRegular() && info.Mode().Perm() == 0600 && stat.Nlink == 1
}
func acquireAdmissionLock(root *os.Root) (*os.File, error) {
	before, err := root.Lstat("admission.lock")
	if err != nil || !privateAdmissionFile(before, false) {
		return nil, ErrUnavailable
	}
	f, err := root.OpenFile("admission.lock", os.O_RDWR, 0)
	if err != nil {
		return nil, ErrUnavailable
	}
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) {
		return nil, errors.Join(ErrUnavailable, f.Close())
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		cause := ErrUnavailable
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			cause = ErrBusy
		}
		return nil, errors.Join(cause, f.Close())
	}
	return f, nil
}
func releaseAdmissionLock(f *os.File) error {
	if f == nil {
		return nil
	}
	return errors.Join(syscall.Flock(int(f.Fd()), syscall.LOCK_UN), f.Close())
}
