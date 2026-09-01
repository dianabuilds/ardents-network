//go:build windows

package credential

import (
	"fmt"
	"path/filepath"
	"syscall"
)

type issuerRootLease struct{ handle syscall.Handle }

func acquireIssuerRootLease(root string) (issuerRootLease, error) {
	path, err := syscall.UTF16PtrFromString(filepath.Join(root, issuerRootLockName))
	if err != nil {
		return issuerRootLease{}, err
	}
	handle, err := syscall.CreateFile(path, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
		syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return issuerRootLease{}, fmt.Errorf("acquire exclusive transit grant issuer lease: %w", err)
	}
	return issuerRootLease{handle: handle}, nil
}

func (lease issuerRootLease) release() error {
	if lease.handle == 0 || lease.handle == syscall.InvalidHandle {
		return nil
	}
	return syscall.CloseHandle(lease.handle)
}
