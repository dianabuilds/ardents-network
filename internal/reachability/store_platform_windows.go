package reachability

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

const storePlatformSupported = true

func retainStoreSyncDirectory(root *os.Root) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(root.Name())
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), root.Name()), nil
}

type storeLease struct{ file *os.File }

func (lease *storeLease) check(expected os.FileInfo) error {
	if lease.file == nil {
		return errors.New("reachability original lease unavailable")
	}
	info, err := lease.file.Stat()
	if err != nil || !os.SameFile(info, expected) {
		return errors.Join(errors.New("reachability original lease lost"), err)
	}
	return nil
}

func acquireStoreLease(root *os.Root) (storeLease, error) {
	name, err := windows.UTF16PtrFromString(filepath.Join(root.Name(), storeLockName))
	if err != nil {
		return storeLease{}, err
	}
	// Read-only identity inspection may coexist. Another writer or deletion
	// still cannot acquire this original lease.
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return storeLease{}, err
	}
	return storeLease{file: os.NewFile(uintptr(handle), filepath.Join(root.Name(), storeLockName))}, nil
}

func (lease *storeLease) release() error {
	if lease == nil || lease.file == nil {
		return nil
	}
	err := lease.file.Close()
	lease.file = nil
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
