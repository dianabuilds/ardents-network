//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package release

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

const historyPlatformSupported = true

type historyLease struct{ file *os.File }

func checkHistoryRoot(info os.FileInfo) error {
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok || s.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0077 != 0 {
		return errors.New("release: history root ownership unavailable")
	}
	return nil
}
func acquireHistoryLease(root string) (historyLease, error) {
	f, err := os.OpenFile(filepath.Join(root, floorStoreLockName), os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return historyLease{}, err
	}
	info, err := f.Stat()
	if err == nil {
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != uint32(os.Geteuid()) || stat.Nlink != 1 || info.Mode().Perm()&0077 != 0 || !info.Mode().IsRegular() {
			err = errors.New("release: invalid lease ownership")
		}
	}
	if err == nil {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	}
	if err != nil {
		return historyLease{}, errors.Join(err, f.Close())
	}
	return historyLease{f}, nil
}
func (l *historyLease) release() error {
	if l.file == nil {
		return nil
	}
	err := errors.Join(syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN), l.file.Close())
	l.file = nil
	return err
}
func durableRename(from, to string) error { return os.Rename(from, to) }
func syncDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(f.Sync(), f.Close())
}
