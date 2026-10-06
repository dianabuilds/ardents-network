package reachability

import (
	"errors"
	"path/filepath"

	"golang.org/x/sys/windows"
)

const storePlatformSupported = true

type storeLease struct{ handle windows.Handle }

func acquireStoreLease(root string) (storeLease, error) {
	name, err := windows.UTF16PtrFromString(filepath.Join(root, storeLockName))
	if err != nil {
		return storeLease{}, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return storeLease{}, err
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

func syncStoreDirectory(path string) error {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	return errors.Join(windows.FlushFileBuffers(handle), windows.CloseHandle(handle))
}
