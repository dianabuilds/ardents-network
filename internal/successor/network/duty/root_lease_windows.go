//go:build windows

package duty

import (
	"errors"
	"fmt"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

type rootLease struct{ handle syscall.Handle }

func acquireRootLeaseMode(root string, create bool) (rootLease, error) {
	disposition := uint32(syscall.OPEN_EXISTING)
	if create {
		disposition = syscall.OPEN_ALWAYS
	}
	path, err := syscall.UTF16PtrFromString(filepath.Join(root, rootLockName))
	if err != nil {
		return rootLease{}, err
	}
	handle, err := syscall.CreateFile(path, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
		disposition, syscall.FILE_ATTRIBUTE_NORMAL|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return rootLease{}, fmt.Errorf("acquire exclusive local role lease: %w", err)
	}
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &info); err != nil || info.FileAttributes&(syscall.FILE_ATTRIBUTE_REPARSE_POINT|syscall.FILE_ATTRIBUTE_DIRECTORY) != 0 || info.NumberOfLinks != 1 {
		return rootLease{}, errors.Join(fmt.Errorf("local role lease is not a regular file"), err, syscall.CloseHandle(handle))
	}
	return rootLease{handle: handle}, nil
}

func (lease rootLease) release() error {
	if lease.handle == 0 || lease.handle == syscall.InvalidHandle {
		return nil
	}
	return syscall.CloseHandle(lease.handle)
}

func rootLeaseBusy(err error) bool { return errors.Is(err, windows.ERROR_SHARING_VIOLATION) }
