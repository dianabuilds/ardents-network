//go:build windows

package namespace

import (
	"fmt"
	"path/filepath"
	"syscall"
)

type namespaceRootLease struct{ handle syscall.Handle }

func acquireNamespaceRootLease(root string) (namespaceRootLease, error) {
	path, err := syscall.UTF16PtrFromString(filepath.Join(root, namespaceRootLockName))
	if err != nil {
		return namespaceRootLease{}, fmt.Errorf("encode naming state-root lease path: %w", err)
	}
	handle, err := syscall.CreateFile(path, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
		syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return namespaceRootLease{}, fmt.Errorf("acquire exclusive naming state-root lease: %w", err)
	}
	return namespaceRootLease{handle: handle}, nil
}

func (lease namespaceRootLease) release() error {
	if lease.handle == 0 || lease.handle == syscall.InvalidHandle {
		return nil
	}
	if err := syscall.CloseHandle(lease.handle); err != nil {
		return fmt.Errorf("release naming state-root lease: %w", err)
	}
	return nil
}

func syncNamespaceDirectory(path string) error {
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
