//go:build windows

package issuer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type issuerRootLease struct {
	handle   syscall.Handle
	root     string
	rootInfo os.FileInfo
}

func acquireIssuerRootLease(root string) (issuerRootLease, error) {
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return issuerRootLease{}, err
	}
	retained, err := issuerRootRetained(root)
	if err != nil {
		return issuerRootLease{}, err
	}
	disposition := uint32(syscall.OPEN_ALWAYS)
	if retained {
		disposition = syscall.OPEN_EXISTING
	}
	path, err := syscall.UTF16PtrFromString(filepath.Join(root, issuerRootLockName))
	if err != nil {
		return issuerRootLease{}, err
	}
	handle, err := syscall.CreateFile(path, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
		disposition, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return issuerRootLease{}, fmt.Errorf("acquire exclusive transit grant issuer lease: %w", err)
	}
	return issuerRootLease{handle: handle, root: root, rootInfo: rootInfo}, nil
}

func (lease issuerRootLease) current() error {
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(lease.handle, &info); err != nil {
		return err
	}
	root, err := os.Lstat(lease.root)
	if err != nil {
		return err
	}
	// The share-zero handle denies replacement/deletion of the open lock.
	if !root.IsDir() || !os.SameFile(root, lease.rootInfo) || info.NumberOfLinks != 1 || info.FileSizeHigh != 0 || info.FileSizeLow != 0 {
		return errors.New("issuer retained lease identity changed")
	}
	return nil
}

func (lease issuerRootLease) release() error {
	if lease.handle == 0 || lease.handle == syscall.InvalidHandle {
		return nil
	}
	return syscall.CloseHandle(lease.handle)
}
