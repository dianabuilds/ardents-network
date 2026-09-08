//go:build windows

package route

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

type closedSpendLease struct {
	file       *os.File
	overlapped windows.Overlapped
}

func acquireClosedSpendLease(path string) (closedSpendLease, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return closedSpendLease{}, err
	}
	lease := closedSpendLease{file: file}
	if err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &lease.overlapped); err != nil {
		_ = file.Close()
		return closedSpendLease{}, errors.New("closed spend journal is already owned")
	}
	return lease, nil
}

func (lease closedSpendLease) release() error {
	if lease.file == nil {
		return nil
	}
	return errors.Join(windows.UnlockFileEx(windows.Handle(lease.file.Fd()), 0, 1, 0, &lease.overlapped), lease.file.Close())
}
