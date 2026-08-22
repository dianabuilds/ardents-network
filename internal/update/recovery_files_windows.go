//go:build windows

package update

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

type recoveryFileIdentity struct {
	volume, links uint32
	index         uint64
	attributes    uint32
	size          int64
}

func recoveryOpen(path string, directory bool) (*os.File, error) {
	encoded, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	flags := uint32(windows.FILE_FLAG_OPEN_REPARSE_POINT)
	if directory {
		flags |= windows.FILE_FLAG_BACKUP_SEMANTICS
	}
	handle, err := windows.CreateFile(encoded, windows.GENERIC_READ,
		windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: open %s: %v", errInventoryInvalid, path, err)
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		return nil, errors.Join(errInventoryInvalid, windows.CloseHandle(handle))
	}
	if err := recoveryRevalidate(file, path, directory, -1); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}

func recoveryRevalidate(file *os.File, path string, directory bool, expectedSize int64) error {
	handle := windows.Handle(file.Fd())
	held, err := recoveryWindowsIdentity(handle)
	if err != nil {
		return err
	}
	current, err := recoveryWindowsPathIdentity(path, directory)
	if err != nil {
		return err
	}
	if held.volume != current.volume || held.index != current.index || held.links != current.links || held.attributes != current.attributes {
		return fmt.Errorf("%w: handle/path identity mismatch", errInventoryInvalid)
	}
	if held.attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("%w: reparse point", errInventoryInvalid)
	}
	isDirectory := held.attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
	if isDirectory != directory {
		return fmt.Errorf("%w: physical kind mismatch", errInventoryInvalid)
	}
	if !directory && held.links != 1 {
		return fmt.Errorf("%w: linked file", errInventoryInvalid)
	}
	if expectedSize >= 0 && held.size != expectedSize {
		return fmt.Errorf("%w: file size changed", errInventoryInvalid)
	}
	return nil
}

func recoveryWindowsPathIdentity(path string, directory bool) (recoveryFileIdentity, error) {
	encoded, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return recoveryFileIdentity{}, err
	}
	flags := uint32(windows.FILE_FLAG_OPEN_REPARSE_POINT)
	if directory {
		flags |= windows.FILE_FLAG_BACKUP_SEMANTICS
	}
	handle, err := windows.CreateFile(encoded, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, flags, 0)
	if err != nil {
		return recoveryFileIdentity{}, err
	}
	identity, identityErr := recoveryWindowsIdentity(handle)
	return identity, errors.Join(identityErr, windows.CloseHandle(handle))
}

func recoveryWindowsIdentity(handle windows.Handle) (recoveryFileIdentity, error) {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return recoveryFileIdentity{}, err
	}
	return recoveryFileIdentity{
		volume: info.VolumeSerialNumber,
		index:  uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow),
		links:  info.NumberOfLinks, attributes: info.FileAttributes,
		size: int64(uint64(info.FileSizeHigh)<<32 | uint64(info.FileSizeLow)),
	}, nil
}
