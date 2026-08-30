//go:build windows

package custody

import (
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

type vaultOperationLock struct{ handle windows.Handle }

func acquireVaultOperationLock(root string) (*vaultOperationLock, error) {
	path := filepath.Join(root, vaultLockName)
	encoded, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, ErrInvalid
	}
	handle, err := windows.CreateFile(encoded, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		if errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return nil, ErrBusy
		}
		return nil, ErrInvalid
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil || info.NumberOfLinks != 1 ||
		info.FileSizeHigh != 0 || info.FileSizeLow != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 || !vaultLockNamesPath(handle, path) {
		_ = windows.CloseHandle(handle)
		return nil, ErrInvalid
	}
	return &vaultOperationLock{handle: handle}, nil
}

func vaultLockNamesPath(handle windows.Handle, expected string) bool {
	buffer := make([]uint16, 512)
	count, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
	if err != nil || count == 0 || count >= uint32(len(buffer)) {
		return false
	}
	path := windows.UTF16ToString(buffer[:count])
	if !strings.HasPrefix(path, `\\?\`) {
		return false
	}
	path = path[len(`\\?\`):]
	if strings.HasPrefix(path, `UNC\`) {
		path = `\\` + path[len(`UNC\`):]
	}
	return strings.EqualFold(filepath.Clean(path), filepath.Clean(expected))
}

func (lock *vaultOperationLock) release() error {
	if lock == nil || lock.handle == windows.InvalidHandle {
		return nil
	}
	err := windows.CloseHandle(lock.handle)
	lock.handle = windows.InvalidHandle
	return err
}
