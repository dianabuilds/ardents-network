package reachability

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

const storePlatformSupported = true

type storeLease struct{ file *os.File }

func acquireStoreLease(root string) (storeLease, error) {
	file, err := os.OpenFile(filepath.Join(root, storeLockName), os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return storeLease{}, err
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return storeLease{}, errors.Join(err, file.Close())
	}
	return storeLease{file: file}, nil
}

func (lease *storeLease) release() error {
	if lease == nil || lease.file == nil {
		return nil
	}
	err := errors.Join(syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN), lease.file.Close())
	lease.file = nil
	return err
}

func syncStoreDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}
