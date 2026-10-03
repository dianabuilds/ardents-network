//go:build !windows

package spending

import (
	"errors"
	"os"
	"syscall"
)

type closedSpendLease struct {
	file    *os.File
	created bool
}

func acquireClosedSpendLease(path string) (closedSpendLease, error) {
	file, created, err := openClosedSpendLeaseFile(path)
	if err != nil {
		return closedSpendLease{}, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return closedSpendLease{}, errors.New("closed spend journal is already owned")
	}
	return closedSpendLease{file: file, created: created}, nil
}

func (lease closedSpendLease) release() error {
	if lease.file == nil {
		return nil
	}
	return errors.Join(syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN), lease.file.Close())
}
