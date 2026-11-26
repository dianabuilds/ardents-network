//go:build windows

package spending

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

type closedSpendLease struct {
	file       *os.File
	created    bool
	overlapped windows.Overlapped
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
	lease := closedSpendLease{file: file, created: created}
	if err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &lease.overlapped); err != nil {
		return closedSpendLease{}, errors.Join(errors.New("closed spend journal is already owned"), err, file.Close())
	}
	return lease, nil
}

func (lease closedSpendLease) release() error {
	if lease.file == nil {
		return nil
	}
	return errors.Join(windows.UnlockFileEx(windows.Handle(lease.file.Fd()), 0, 1, 0, &lease.overlapped), lease.file.Close())
}
