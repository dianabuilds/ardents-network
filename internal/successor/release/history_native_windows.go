package release

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
)

const historyPlatformSupported = true

type historyLease struct{ handle windows.Handle }

func checkHistoryRoot(os.FileInfo) error { return nil }
func acquireHistoryLease(root string) (historyLease, error) {
	name, err := windows.UTF16PtrFromString(filepath.Join(root, floorStoreLockName))
	if err != nil {
		return historyLease{}, err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return historyLease{}, err
	}
	var info windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(h, &info)
	if err == nil && (info.NumberOfLinks != 1 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0) {
		err = errors.New("release: invalid lease identity")
	}
	if err != nil {
		return historyLease{}, errors.Join(err, windows.CloseHandle(h))
	}
	return historyLease{h}, nil
}
func (l *historyLease) release() error {
	if l.handle == windows.InvalidHandle {
		return nil
	}
	err := windows.CloseHandle(l.handle)
	l.handle = windows.InvalidHandle
	return err
}
func durableRename(from, to string) error {
	a, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	b, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(a, b, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
func syncDirectory(path string) error {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	return errors.Join(windows.FlushFileBuffers(h), windows.CloseHandle(h))
}
