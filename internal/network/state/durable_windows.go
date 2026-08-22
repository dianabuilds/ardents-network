//go:build windows

package state

import (
	"fmt"
	"path/filepath"
	"syscall"
)

type rootLease struct{ handle syscall.Handle }

func acquireRootLease(root string) (rootLease, error) {
	path, err := syscall.UTF16PtrFromString(filepath.Join(root, rootLockName))
	if err != nil {
		return rootLease{}, fmt.Errorf("encode state-root lease path: %w", err)
	}
	handle, err := syscall.CreateFile(path, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
		syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return rootLease{}, fmt.Errorf("acquire exclusive state-root lease: %w", err)
	}
	return rootLease{handle: handle}, nil
}

func (lease rootLease) release() error {
	if lease.handle == 0 || lease.handle == syscall.InvalidHandle {
		return nil
	}
	if err := syscall.CloseHandle(lease.handle); err != nil {
		return fmt.Errorf("release state-root lease: %w", err)
	}
	return nil
}

func syncDirectory(path string) error {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil,
		syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	flushErr := syscall.FlushFileBuffers(handle)
	closeErr := syscall.CloseHandle(handle)
	if flushErr != nil {
		return flushErr
	}
	return closeErr
}
