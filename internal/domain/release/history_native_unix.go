//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package release

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

const historyPlatformSupported = true

type historyLease struct {
	file      *os.File
	directory *os.File
	original  os.FileInfo
}

func checkHistoryRoot(info os.FileInfo) error {
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok || s.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0077 != 0 {
		return errors.New("release: history root ownership unavailable")
	}
	return nil
}
func acquireHistoryLease(root string) (historyLease, error) {
	d, err := os.OpenFile(root, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_DIRECTORY, 0)
	if err != nil {
		return historyLease{}, err
	}
	original, err := d.Stat()
	if err == nil {
		err = checkHistoryRoot(original)
	}
	if err != nil {
		return historyLease{}, errors.Join(err, d.Close())
	}
	f, err := os.OpenFile(filepath.Join(root, floorStoreLockName), os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return historyLease{}, errors.Join(err, d.Close())
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
	if err == nil {
		named, e := os.Lstat(root)
		if e != nil || !os.SameFile(original, named) {
			err = errors.Join(ErrTrustUnavailable, e)
		}
	}
	if err != nil {
		return historyLease{}, errors.Join(err, f.Close(), d.Close())
	}
	return historyLease{file: f, directory: d, original: original}, nil
}
func (l *historyLease) release() error {
	if l.file == nil {
		return nil
	}
	err := errors.Join(syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN), l.file.Close(), l.directory.Close())
	l.file = nil
	l.directory = nil
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
