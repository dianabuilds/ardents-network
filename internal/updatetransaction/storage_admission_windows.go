//go:build windows

package updatetransaction

import (
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// observeOwnedStorage admits only a fixed NTFS volume. Windows has no
// portable free-object counter, so itemsKnown remains false; a later object
// creation failure is a staging failure rather than a fabricated inode claim.
func observeOwnedStorage(root string) (resourceObservation, error) {
	volumeRoot := filepath.VolumeName(root) + `\`
	encoded, err := windows.UTF16PtrFromString(volumeRoot)
	if err != nil || windows.GetDriveType(encoded) != windows.DRIVE_FIXED {
		return resourceObservation{}, errors.New("unsupported windows update storage")
	}
	var serial, maximumComponent, flags uint32
	fileSystem := make([]uint16, 32)
	if err := windows.GetVolumeInformation(encoded, nil, 0, &serial, &maximumComponent, &flags, &fileSystem[0], uint32(len(fileSystem))); err != nil ||
		!strings.EqualFold(windows.UTF16ToString(fileSystem), "NTFS") {
		return resourceObservation{}, errors.New("unsupported windows update storage")
	}
	if err := validateWindowsVolumeIdentity(root, serial); err != nil {
		return resourceObservation{}, err
	}
	if err := validateWindowsOwnerAndDACL(root); err != nil {
		return resourceObservation{}, err
	}
	var available, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(encoded, &available, &total, &free); err != nil {
		return resourceObservation{}, errors.Join(errCapacityObservation, err)
	}
	// GetDiskFreeSpaceEx provides the per-caller byte availability but not an
	// allocation-unit value without raw syscall pointer conversions. Treating
	// each required file as byte-granular is a lower-bound observation: it never
	// claims a quota or a free-inode guarantee, and a later allocation refusal
	// remains a staging failure.
	return resourceObservation{allocationUnit: 1, availableBytes: available}, nil
}

func validateWindowsOwnerAndDACL(root string) error {
	descriptor, err := windows.GetNamedSecurityInfo(root, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := descriptor.Owner()
	if err != nil || owner == nil {
		return errors.New("windows root owner is unavailable")
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil || !owner.Equals(user.User.Sid) {
		return errors.New("windows root owner is unsupported")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil || dacl == nil {
		return errors.New("windows root DACL is unavailable")
	}
	return nil
}

func validateWindowsVolumeIdentity(root string, serial uint32) error {
	encoded, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(encoded, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	var information windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &information); err != nil ||
		information.VolumeSerialNumber != serial || information.NumberOfLinks == 0 {
		return errors.New("windows volume identity is unsupported")
	}
	return nil
}
