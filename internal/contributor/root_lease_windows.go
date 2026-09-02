//go:build windows

package contributor

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"

	"golang.org/x/sys/windows"
)

type rootLease struct {
	mutex       windows.Handle
	ownerThread bool
}

type rootLeaseDirectoryID struct {
	volumeSerial uint32
	fileIndexHi  uint32
	fileIndexLo  uint32
}

func acquireRootLeaseDirectory(path string) (rootLease, error) {
	directoryID, err := contributorLeaseDirectoryID(path)
	if err != nil {
		return rootLease{}, err
	}
	name, err := windows.UTF16PtrFromString(contributorLeaseMutexName(directoryID))
	if err != nil {
		return rootLease{}, fmt.Errorf("encode Contributor root-lease mutex name: %w", err)
	}
	handle, createErr := windows.CreateMutex(nil, false, name)
	if handle == 0 {
		if createErr != nil {
			return rootLease{}, fmt.Errorf("create Contributor root-lease mutex: %w", createErr)
		}
		return rootLease{}, errors.New("create Contributor root-lease mutex returned no handle")
	}
	if createErr != nil && !errors.Is(createErr, windows.ERROR_ALREADY_EXISTS) {
		return rootLease{}, errors.Join(
			fmt.Errorf("open Contributor root-lease mutex: %w", createErr),
			wrapRootLeaseRelease("close", windows.CloseHandle(handle)),
		)
	}

	// Windows mutex ownership belongs to an OS thread. Apply and Control defer
	// release on this same goroutine, so pinning it keeps ReleaseMutex valid.
	runtime.LockOSThread()
	ownedThread := true
	defer func() {
		if ownedThread {
			runtime.UnlockOSThread()
		}
	}()
	state, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return rootLease{}, errors.Join(
			fmt.Errorf("wait for Contributor root lease: %w", err),
			wrapRootLeaseRelease("close", windows.CloseHandle(handle)),
		)
	}
	switch state {
	case uint32(windows.WAIT_OBJECT_0), uint32(windows.WAIT_ABANDONED):
		ownedThread = false
		return rootLease{mutex: handle, ownerThread: true}, nil
	case uint32(windows.WAIT_TIMEOUT):
		return rootLease{}, errors.Join(errContributorRootBusy, wrapRootLeaseRelease("close", windows.CloseHandle(handle)))
	default:
		return rootLease{}, errors.Join(
			fmt.Errorf("wait for Contributor root lease returned %#x", state),
			wrapRootLeaseRelease("close", windows.CloseHandle(handle)),
		)
	}
}

func contributorLeaseDirectoryID(path string) (identity rootLeaseDirectoryID, resultErr error) {
	encoded, err := windows.UTF16PtrFromString(filepath.Clean(path))
	if err != nil {
		return rootLeaseDirectoryID{}, fmt.Errorf("encode Contributor root-lease directory: %w", err)
	}
	handle, err := windows.CreateFile(encoded, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return rootLeaseDirectoryID{}, fmt.Errorf("open Contributor root-lease directory: %w", err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, wrapRootLeaseRelease("close directory", windows.CloseHandle(handle)))
	}()
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return rootLeaseDirectoryID{}, fmt.Errorf("inspect Contributor root-lease directory: %w", err)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return rootLeaseDirectoryID{}, errors.New("contributor root-lease directory is invalid")
	}
	return rootLeaseDirectoryID{volumeSerial: info.VolumeSerialNumber, fileIndexHi: info.FileIndexHigh, fileIndexLo: info.FileIndexLow}, nil
}

func contributorLeaseMutexName(identity rootLeaseDirectoryID) string {
	// Global scope excludes concurrent operator commands from distinct Windows
	// sessions. The physical directory identity prevents alias paths splitting
	// the lease, and the inspection handle closes before lifecycle work begins.
	return fmt.Sprintf(`Global\ardents-contributor-root-%08x-%08x-%08x`, identity.volumeSerial, identity.fileIndexHi, identity.fileIndexLo)
}

func (lease rootLease) release() error {
	if lease.mutex == 0 || lease.mutex == windows.InvalidHandle {
		return nil
	}
	if lease.ownerThread {
		defer runtime.UnlockOSThread()
	}
	return errors.Join(
		wrapRootLeaseRelease("unlock", windows.ReleaseMutex(lease.mutex)),
		wrapRootLeaseRelease("close", windows.CloseHandle(lease.mutex)),
	)
}
