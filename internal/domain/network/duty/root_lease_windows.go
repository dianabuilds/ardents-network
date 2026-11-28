//go:build windows

package duty

import (
	"errors"
	"fmt"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/durablefile"

	"golang.org/x/sys/windows"
)

type rootLease struct{ handle syscall.Handle }

func acquireRootLeaseMode(root string, create bool) (rootLease, error) {
	disposition := uint32(syscall.OPEN_EXISTING)
	if create {
		disposition = syscall.OPEN_ALWAYS
	}
	return openRootLease(root, disposition)
}

func acquireFreshRootLease(root string) (rootLease, error) {
	return openRootLease(root, syscall.CREATE_NEW)
}

func openRootLease(root string, disposition uint32) (rootLease, error) {
	handle, err := durablefile.OpenOwnerOnlyLease(filepath.Join(root, rootLockName), disposition)
	if err != nil {
		return rootLease{}, fmt.Errorf("acquire exclusive local role lease: %w", err)
	}
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(handle), &info); err != nil || info.FileAttributes&(syscall.FILE_ATTRIBUTE_REPARSE_POINT|syscall.FILE_ATTRIBUTE_DIRECTORY) != 0 || info.NumberOfLinks != 1 {
		return rootLease{}, errors.Join(fmt.Errorf("local role lease is not a regular file"), err, syscall.CloseHandle(syscall.Handle(handle)))
	}
	return rootLease{handle: syscall.Handle(handle)}, nil
}

func (lease rootLease) release() error {
	if lease.handle == 0 || lease.handle == syscall.InvalidHandle {
		return nil
	}
	return syscall.CloseHandle(lease.handle)
}

func rootLeaseBusy(err error) bool { return errors.Is(err, windows.ERROR_SHARING_VIOLATION) }
