//go:build windows

package publication

import (
	"errors"
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

type rootLease struct{ handle windows.Handle }

func acquireRootLease(root string) (rootLease, error) {
	path, err := windows.UTF16PtrFromString(filepath.Join(root, rootLockName))
	if err != nil {
		return rootLease{}, err
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		if errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return rootLease{}, errors.New("publication root is already owned")
		}
		return rootLease{}, fmt.Errorf("open publication root lease: %w", err)
	}
	return rootLease{handle: handle}, nil
}

func (lease *rootLease) release() error {
	if lease == nil || lease.handle == windows.InvalidHandle {
		return nil
	}
	err := windows.CloseHandle(lease.handle)
	lease.handle = windows.InvalidHandle
	return err
}
