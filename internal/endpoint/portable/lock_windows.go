//go:build windows

package portable

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

type ownerLease struct {
	file       *os.File
	overlapped windows.Overlapped
}

func acquireOwnerLease(path string) (ownerLease, error) {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return ownerLease{}, lifecycleError(ReasonLockError, errors.New("owner lock is not a regular file"))
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return ownerLease{}, lifecycleError(ReasonLockError, err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return ownerLease{}, lifecycleError(ReasonLockError, err)
	}
	if err := setOwnerOnlyDACL(path, windows.NO_INHERITANCE); err != nil {
		_ = file.Close()
		return ownerLease{}, lifecycleError(ReasonLockError, err)
	}
	lease := ownerLease{file: file}
	if err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, &lease.overlapped); err != nil {
		_ = file.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return ownerLease{}, lifecycleError(ReasonOwnerBusy, err)
		}
		return ownerLease{}, lifecycleError(ReasonLockError, err)
	}
	return lease, nil
}

func (lease ownerLease) release() error {
	if lease.file == nil {
		return nil
	}
	return errors.Join(windows.UnlockFileEx(windows.Handle(lease.file.Fd()), 0, 1, 0, &lease.overlapped), lease.file.Close())
}
