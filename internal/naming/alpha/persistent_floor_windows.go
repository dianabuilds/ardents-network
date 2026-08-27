//go:build windows

package alpha

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

type persistentFloorLease struct{ handle windows.Handle }

func acquirePersistentFloorLease(root string) (persistentFloorLease, error) {
	path, err := windows.UTF16PtrFromString(filepath.Join(root, persistentFloorLock))
	if err != nil {
		return persistentFloorLease{}, fmt.Errorf("encode alpha persistent floor lease path: %w", err)
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return persistentFloorLease{}, fmt.Errorf("acquire alpha persistent floor lease: %w", err)
	}
	return persistentFloorLease{handle: handle}, nil
}

func (lease persistentFloorLease) release() error {
	if lease.handle == 0 || lease.handle == windows.InvalidHandle {
		return nil
	}
	return windows.CloseHandle(lease.handle)
}

func durablePersistentFloorRename(oldPath, newPath string) error {
	oldPointer, err := windows.UTF16PtrFromString(oldPath)
	if err != nil {
		return err
	}
	newPointer, err := windows.UTF16PtrFromString(newPath)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(oldPointer, newPointer, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

func syncPersistentFloorDirectory(string) error { return nil }
