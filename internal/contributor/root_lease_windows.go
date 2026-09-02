//go:build windows

package contributor

import (
	"errors"
	"fmt"
	"path/filepath"
	"syscall"
)

type rootLease struct{ handle syscall.Handle }

const windowsErrorSharingViolation = syscall.Errno(32)

func acquireRootLeaseFile(path string) (rootLease, error) {
	encoded, err := syscall.UTF16PtrFromString(filepath.Clean(path))
	if err != nil {
		return rootLease{}, err
	}
	handle, err := syscall.CreateFile(encoded, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
		syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		if errors.Is(err, windowsErrorSharingViolation) {
			return rootLease{}, errContributorRootBusy
		}
		return rootLease{}, fmt.Errorf("acquire exclusive Contributor root lease: %w", err)
	}
	return rootLease{handle: handle}, nil
}

func (lease rootLease) release() error {
	if lease.handle == 0 || lease.handle == syscall.InvalidHandle {
		return nil
	}
	return syscall.CloseHandle(lease.handle)
}
