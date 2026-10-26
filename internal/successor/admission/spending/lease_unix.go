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
	return lockClosedSpendLease(file, created)
}

func acquireRetainedClosedSpendLease(path string) (closedSpendLease, error) {
	file, err := openRetainedClosedSpendLeaseFile(path)
	if err != nil {
		return closedSpendLease{}, err
	}
	return lockClosedSpendLease(file, false)
}

func lockClosedSpendLease(file *os.File, created bool) (closedSpendLease, error) {
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return closedSpendLease{}, errors.Join(errors.New("closed spend journal is already owned"), err, file.Close())
	}
	return closedSpendLease{file: file, created: created}, nil
}

func (lease closedSpendLease) release() error {
	if lease.file == nil {
		return nil
	}
	return errors.Join(syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN), lease.file.Close())
}
