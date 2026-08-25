//go:build windows

package alphacontrol

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

type readerLease struct{ handle windows.Handle }

func acquireReaderLease(root string) (readerLease, error) {
	path, err := windows.UTF16PtrFromString(filepath.Join(root, readerLockName))
	if err != nil {
		return readerLease{}, fmt.Errorf("encode alpha control reader lease path: %w", err)
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return readerLease{}, fmt.Errorf("acquire alpha control reader lease: %w", err)
	}
	return readerLease{handle: handle}, nil
}

func (lease readerLease) release() error {
	if lease.handle == 0 || lease.handle == windows.InvalidHandle {
		return nil
	}
	return windows.CloseHandle(lease.handle)
}
