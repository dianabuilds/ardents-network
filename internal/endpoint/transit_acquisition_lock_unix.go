//go:build !windows

package endpoint

import (
	"errors"
	"os"
	"syscall"
)

type transitAcquisitionLease struct{ file *os.File }

func acquireTransitAcquisitionLease(path string) (transitAcquisitionLease, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return transitAcquisitionLease{}, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return transitAcquisitionLease{}, errors.New("transit acquisition root is already owned")
	}
	return transitAcquisitionLease{file: file}, nil
}

func (lease transitAcquisitionLease) release() error {
	if lease.file == nil {
		return nil
	}
	return errors.Join(syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN), lease.file.Close())
}
