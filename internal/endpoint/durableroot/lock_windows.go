//go:build windows

package durableroot

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

type Lease struct {
	file       *os.File
	overlapped windows.Overlapped
}

func Acquire(path string) (*Lease, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	lease := &Lease{file: file}
	if err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, &lease.overlapped); err != nil {
		_ = file.Close()
		return nil, errors.New("Endpoint root is already owned")
	}
	return lease, nil
}

func (lease *Lease) Release() error {
	if lease == nil || lease.file == nil {
		return nil
	}
	return errors.Join(windows.UnlockFileEx(windows.Handle(lease.file.Fd()), 0, 1, 0, &lease.overlapped), lease.file.Close())
}
