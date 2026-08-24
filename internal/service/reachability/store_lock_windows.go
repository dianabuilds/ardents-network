//go:build windows

package reachability

import (
	"errors"
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

type storeLease struct{ handle windows.Handle }

func acquireStoreLease(root string) (storeLease, error) {
	path, err := windows.UTF16PtrFromString(filepath.Join(root, storeLockName))
	if err != nil {
		return storeLease{}, err
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		if errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return storeLease{}, errors.New("reachability store is already owned")
		}
		return storeLease{}, fmt.Errorf("open reachability store lease: %w", err)
	}
	return storeLease{handle: handle}, nil
}

func (lease *storeLease) release() error {
	if lease == nil || lease.handle == windows.InvalidHandle {
		return nil
	}
	err := windows.CloseHandle(lease.handle)
	lease.handle = windows.InvalidHandle
	return err
}
