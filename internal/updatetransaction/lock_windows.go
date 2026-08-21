//go:build windows

package updatetransaction

import (
	"errors"
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

var errLockBusy = errors.New("update transaction busy")
var errLockIdentity = errors.New("update transaction lock identity is invalid")

const lockFileName = ".ardents-update-transaction-lock"

type ownedLock struct {
	path   string
	handle windows.Handle
}

// acquireOwnedLock opens the existing permanent lock with
// OPEN_EXISTING plus FILE_FLAG_OPEN_REPARSE_POINT and zero share mode,
// observes identity through the held handle plus a non-reparse path
// handle, and calls non-blocking LockFileEx. The zero-share mode is
// the platform exclusion primitive: ERROR_SHARING_VIOLATION during
// the one open and ERROR_LOCK_VIOLATION during LockFileEx are the
// only Windows busy classifications. ACL/access, missing, reparse,
// directory, non-empty, linked, or other failures are
// invalid/unsupported evidence, never busy. The lock is never
// created, repaired, replaced, retried, or unlinked by the Module.
func acquireOwnedLock(root string) (*ownedLock, error) {
	lockPath := filepath.Join(root, lockFileName)
	encoded, err := windows.UTF16PtrFromString(lockPath)
	if err != nil {
		return nil, fmt.Errorf("%w: encode path: %v", errLockIdentity, err)
	}
	handle, openErr := windows.CreateFile(encoded, windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if openErr != nil {
		if errors.Is(openErr, windows.ERROR_SHARING_VIOLATION) {
			return nil, fmt.Errorf("%w: %v", errLockBusy, openErr)
		}
		return nil, fmt.Errorf("%w: open: %v", errLockIdentity, openErr)
	}
	var lockOverlapped windows.Overlapped
	lockErr := windows.LockFileEx(handle, windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &lockOverlapped)
	if lockErr != nil {
		closeErr := windows.CloseHandle(handle)
		if errors.Is(lockErr, windows.ERROR_LOCK_VIOLATION) {
			return nil, errors.Join(fmt.Errorf("%w: %v", errLockBusy, lockErr), closeErr)
		}
		return nil, errors.Join(fmt.Errorf("%w: lock: %v", errLockIdentity, lockErr), closeErr)
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		unlockErr := windows.UnlockFileEx(handle, 0, 1, 0, &lockOverlapped)
		closeErr := windows.CloseHandle(handle)
		return nil, errors.Join(fmt.Errorf("%w: handle: %v", errLockIdentity, err), unlockErr, closeErr)
	}
	if info.NumberOfLinks != 1 || info.FileSizeHigh != 0 || info.FileSizeLow != 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		unlockErr := windows.UnlockFileEx(handle, 0, 1, 0, &lockOverlapped)
		closeErr := windows.CloseHandle(handle)
		return nil, errors.Join(fmt.Errorf("%w: held lock shape is invalid", errLockIdentity), unlockErr, closeErr)
	}
	return &ownedLock{path: lockPath, handle: handle}, nil
}

// release joins UnlockFileEx and CloseHandle errors without ever
// removing or replacing the permanent lock path. The installer or
// portable bootstrap owns the lock's lifecycle; this Module only
// observes failures.
func (l *ownedLock) release() error {
	if l == nil || l.handle == windows.InvalidHandle {
		return nil
	}
	var overlapped windows.Overlapped
	unlockErr := windows.UnlockFileEx(l.handle, 0, 1, 0, &overlapped)
	closeErr := windows.CloseHandle(l.handle)
	l.handle = windows.InvalidHandle
	return errors.Join(unlockErr, closeErr)
}
