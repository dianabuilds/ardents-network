//go:build windows

package instance

import (
	"errors"
	"path/filepath"

	"golang.org/x/sys/windows"
)

type rootLock struct{ handle windows.Handle }

func acquireRootLock(path string) (*rootLock, error) {
	encoded, err := windows.UTF16PtrFromString(filepath.Clean(path))
	if err != nil {
		return nil, ErrInvalid
	}
	handle, err := windows.CreateFile(encoded, windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		if errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return nil, ErrBusy
		}
		return nil, ErrInvalid
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil || info.NumberOfLinks != 1 ||
		info.FileSizeHigh != 0 || info.FileSizeLow != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		_ = windows.CloseHandle(handle)
		return nil, ErrInvalid
	}
	return &rootLock{handle: handle}, nil
}

func (lock *rootLock) release() error {
	if lock == nil || lock.handle == windows.InvalidHandle {
		return nil
	}
	err := windows.CloseHandle(lock.handle)
	lock.handle = windows.InvalidHandle
	return err
}
