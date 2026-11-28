//go:build windows

package selection

import (
	"errors"
	"fmt"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/durablefile"
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
	handle, err := durablefile.OpenOwnerOnlyLease(filepath.Join(root, rootLockName), disposition)
	if err != nil {
		return rootLease{}, fmt.Errorf("acquire exclusive Entry state lease: %w", err)
	}
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(handle), &info); err != nil || info.FileAttributes&(syscall.FILE_ATTRIBUTE_REPARSE_POINT|syscall.FILE_ATTRIBUTE_DIRECTORY) != 0 || info.NumberOfLinks != 1 {
		return rootLease{}, errors.Join(errors.New("Entry state lease is not a regular file"), err, syscall.CloseHandle(syscall.Handle(handle)))
	}
	return rootLease{handle: syscall.Handle(handle)}, nil
}

func (lease rootLease) release() error {
	if lease.handle == 0 || lease.handle == syscall.InvalidHandle {
		return nil
	}
	return syscall.CloseHandle(lease.handle)
}
