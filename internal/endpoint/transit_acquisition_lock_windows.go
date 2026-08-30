//go:build windows

package endpoint

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

type transitAcquisitionLease struct {
	file       *os.File
	overlapped windows.Overlapped
}

func acquireTransitAcquisitionLease(path string) (transitAcquisitionLease, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return transitAcquisitionLease{}, err
	}
	lease := transitAcquisitionLease{file: file}
	if err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, &lease.overlapped); err != nil {
		_ = file.Close()
		return transitAcquisitionLease{}, errors.New("transit acquisition root is already owned")
	}
	return lease, nil
}

func (lease transitAcquisitionLease) release() error {
	if lease.file == nil {
		return nil
	}
	return errors.Join(windows.UnlockFileEx(windows.Handle(lease.file.Fd()), 0, 1, 0, &lease.overlapped), lease.file.Close())
}
