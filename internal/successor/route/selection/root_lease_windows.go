//go:build windows

package selection

import (
	"errors"
	"fmt"
	"path/filepath"
	"syscall"
)

// The Windows lease holds exclusive access to the same fixed lock file through
// a zero sharing mode rather than flock. The portable selection callers keep the
// same acquire/release contract; the lease file identity is checked before the
// handle is retained, and a reparse point or directory is never followed.
type rootLease struct{ handle syscall.Handle }

func acquireRootLease(root string) (rootLease, error) {
	return openRootLease(root, syscall.OPEN_ALWAYS)
}

func acquireFreshRootLease(root string) (rootLease, error) {
	return openRootLease(root, syscall.CREATE_NEW)
}

// Retained selection history must never regain a missing lease file by opening.
func acquireRetainedRootLease(root string) (rootLease, error) {
	return openRootLease(root, syscall.OPEN_EXISTING)
}

func openRootLease(root string, disposition uint32) (rootLease, error) {
	path, err := syscall.UTF16PtrFromString(filepath.Join(root, rootLockName))
	if err != nil {
		return rootLease{}, err
	}
	handle, err := syscall.CreateFile(path, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
		disposition, syscall.FILE_ATTRIBUTE_NORMAL|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return rootLease{}, fmt.Errorf("acquire exclusive Entry state lease: %w", err)
	}
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &info); err != nil || info.FileAttributes&(syscall.FILE_ATTRIBUTE_REPARSE_POINT|syscall.FILE_ATTRIBUTE_DIRECTORY) != 0 || info.NumberOfLinks != 1 {
		return rootLease{}, errors.Join(errors.New("Entry state lease is not a regular file"), err, syscall.CloseHandle(handle))
	}
	return rootLease{handle: handle}, nil
}

func (lease rootLease) release() error {
	if lease.handle == 0 || lease.handle == syscall.InvalidHandle {
		return nil
	}
	return syscall.CloseHandle(lease.handle)
}
