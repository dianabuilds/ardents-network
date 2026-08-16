//go:build windows

package bridge

import (
	"fmt"
	"path/filepath"
	"syscall"
)

type rootLease struct{ handle syscall.Handle }

func acquireRootLease(root string) (rootLease, error) {
	path, err := syscall.UTF16PtrFromString(filepath.Join(root, rootLockName))
	if err != nil {
		return rootLease{}, err
	}
	handle, err := syscall.CreateFile(path, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
		syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return rootLease{}, fmt.Errorf("acquire exclusive Bridge state lease: %w", err)
	}
	return rootLease{handle: handle}, nil
}

func (lease rootLease) release() error {
	if lease.handle == 0 || lease.handle == syscall.InvalidHandle {
		return nil
	}
	return syscall.CloseHandle(lease.handle)
}
