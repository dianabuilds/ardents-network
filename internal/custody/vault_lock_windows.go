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
	final, ok := vaultHandleFinalPath(handle)
	if !ok {
		return false
	}
	// The expected path may carry 8.3 short-name components (for example a
	// runner temp directory such as RUNNER~1) while the handle's final path is
	// the normalized long form. Expand the expected path to its long form before
	// comparing so a legitimate lock file is not refused. The comparison still
	// binds the handle to the exact expected path: a substituted target keeps a
	// different final path, and the reparse-point/NumberOfLinks checks above
	// remain unchanged.
	long, err := windowsLongPath(expected)
	if err != nil {
		return false
	}
	return strings.EqualFold(filepath.Clean(final), filepath.Clean(long))
}

// vaultHandleFinalPath returns the normalized DOS path of the open handle with
// the \\?\ prefix removed and UNC paths restored to \\server\share form.
func vaultHandleFinalPath(handle windows.Handle) (string, bool) {
	buffer := make([]uint16, 512)
	for {
		count, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
		if err != nil || count == 0 {
			return "", false
		}
		if count < uint32(len(buffer)) {
			path := windows.UTF16ToString(buffer[:count])
			if !strings.HasPrefix(path, `\\?\`) {
				return "", false
			}
			path = path[len(`\\?\`):]
			if strings.HasPrefix(path, `UNC\`) {
				path = `\\` + path[len(`UNC\`):]
			}
			return path, true
		}
		buffer = make([]uint16, count+1)
	}
}

// windowsLongPath expands any 8.3 short-name components of an existing path to
// its long form, growing the buffer as needed.
func windowsLongPath(path string) (string, error) {
	encoded, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	buffer := make([]uint16, 512)
	for {
		count, err := windows.GetLongPathName(encoded, &buffer[0], uint32(len(buffer)))
		if err != nil {
			return "", err
		}
		if count < uint32(len(buffer)) {
			return windows.UTF16ToString(buffer[:count]), nil
		}
		buffer = make([]uint16, count)
	}
}

func (lock *vaultOperationLock) release() error {
	if lock == nil || lock.handle == windows.InvalidHandle {
		return nil
	}
	err := windows.CloseHandle(lock.handle)
	lock.handle = windows.InvalidHandle
	return err
}
