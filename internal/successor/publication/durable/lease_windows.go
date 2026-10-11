package durable

import (
	"errors"
	"path/filepath"

	"golang.org/x/sys/windows"
)

const platformSupported = true

type rootLease struct{ handle windows.Handle }

func acquireLease(path string) (rootLease, error) {
	name, err := windows.UTF16PtrFromString(filepath.Join(path, lockName))
	if err != nil {
		return rootLease{}, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return rootLease{}, err
	}
	var info windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(handle, &info); err != nil || info.NumberOfLinks != 1 || info.FileSizeHigh != 0 || info.FileSizeLow != 0 || info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		return rootLease{}, errors.Join(errors.New("publication lease invalid"), err, windows.CloseHandle(handle))
	}
	return rootLease{handle: handle}, nil
}
func (lease *rootLease) release() error {
	if lease.handle == windows.InvalidHandle {
		return nil
	}
	err := windows.CloseHandle(lease.handle)
	lease.handle = windows.InvalidHandle
	return err
}

func (lease *rootLease) check(path string) error {
	var original, current windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(lease.handle, &original); err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(filepath.Join(path, lockName))
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(name, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	readErr := windows.GetFileInformationByHandle(handle, &current)
	closeErr := windows.CloseHandle(handle)
	if readErr != nil || closeErr != nil {
		return errors.Join(readErr, closeErr)
	}
	if original.VolumeSerialNumber != current.VolumeSerialNumber || original.FileIndexHigh != current.FileIndexHigh || original.FileIndexLow != current.FileIndexLow || current.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		return errors.New("publication original lease replaced")
	}
	return nil
}
func syncDirectory(path string) error {
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
