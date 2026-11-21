package durable

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

const platformSupported = true

type rootLease struct{ file *os.File }

func acquireLease(path string) (rootLease, error) {
	file, err := os.OpenFile(filepath.Join(path, lockName), os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return rootLease{}, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != 0 {
		return rootLease{}, errors.Join(errors.New("publication lease invalid"), err, file.Close())
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 || stat.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0077 != 0 {
		return rootLease{}, errors.Join(errors.New("publication lease access invalid"), file.Close())
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return rootLease{}, errors.Join(err, file.Close())
	}
	return rootLease{file: file}, nil
}
func (lease *rootLease) release() error {
	if lease.file == nil {
		return nil
	}
	err := errors.Join(syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN), lease.file.Close())
	lease.file = nil
	return err
}

func (lease *rootLease) check(path string) error {
	if lease.file == nil {
		return errors.New("publication lease closed")
	}
	info, err := lease.file.Stat()
	if err != nil {
		return err
	}
	current, err := os.Lstat(filepath.Join(path, lockName))
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(info, current) {
		return errors.New("publication original lease replaced")
	}
	return nil
}
func syncDirectory(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(file.Sync(), file.Close())
}
