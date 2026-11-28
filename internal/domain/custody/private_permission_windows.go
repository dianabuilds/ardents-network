package custody

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"golang.org/x/sys/windows"
)

const windowsFileAllAccess = 0x001f01ff

// Preflight proves the selected parent volume and an empty original staging
// object can carry the required descriptor before opening the Vault. It is not
// a reservation; publication repeats the same checks on its own original file.
func checkPrivatePermissionDestination(path string) error {
	destination, err := filepath.Abs(path)
	if err != nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	parent := filepath.Dir(destination)
	name, err := windows.UTF16PtrFromString(parent)
	if err != nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	var info windows.ByHandleFileInformation
	infoErr := windows.GetFileInformationByHandle(handle, &info)
	if infoErr != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return errors.Join(ErrPrivatePermissionUnavailable, infoErr, windows.CloseHandle(handle))
	}
	if err := checkPrivatePermissionVolume(handle); err != nil {
		return errors.Join(err, windows.CloseHandle(handle))
	}
	if _, err := os.Lstat(destination); err == nil {
		existing, openErr := openCredentialFile(destination, true)
		if openErr != nil {
			return errors.Join(ErrPrivatePermissionUnavailable, openErr, windows.CloseHandle(handle))
		}
		opened, statErr := existing.Stat()
		var validation error
		if statErr != nil || !privatePermissionMetadata(opened) || !sameCredentialFile(existing, destination) {
			validation = errors.Join(ErrPrivatePermissionUnavailable, statErr)
		} else {
			validation = checkPrivatePermissionProtection(existing)
		}
		if err := errors.Join(validation, existing.Close()); err != nil {
			return errors.Join(ErrPrivatePermissionUnavailable, err, windows.CloseHandle(handle))
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.Join(ErrPrivatePermissionUnavailable, err, windows.CloseHandle(handle))
	}
	probe, err := createCredentialStaging(parent, ".ardents-permission-check-", true)
	if err != nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err, windows.CloseHandle(handle))
	}
	opened, statErr := probe.Stat()
	var validation error
	if statErr != nil || !privatePermissionMetadata(opened) || !sameCredentialFile(probe, probe.Name()) {
		validation = errors.Join(ErrPrivatePermissionUnavailable, statErr)
	} else {
		validation = checkPrivatePermissionProtection(probe)
	}
	if err := errors.Join(validation, closeCredentialStaging(probe, probe.Name(), true, true), windows.CloseHandle(handle)); err != nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	return nil
}

func privatePermissionMetadata(info os.FileInfo) bool {
	return info != nil && info.Mode().IsRegular()
}

func checkPrivatePermissionVolume(handle windows.Handle) error {
	var flags uint32
	var filesystem [32]uint16
	if err := windows.GetVolumeInformationByHandle(handle, nil, 0, nil, nil, &flags, &filesystem[0], uint32(len(filesystem))); err != nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	if windows.UTF16ToString(filesystem[:]) != "NTFS" || flags&windows.FILE_PERSISTENT_ACLS == 0 || flags&windows.FILE_SUPPORTS_HARD_LINKS == 0 {
		return ErrPrivatePermissionUnavailable
	}
	return nil
}

// checkPrivatePermissionProtection observes the held object's owner and DACL.
// No existing object's descriptor is changed to make a retry succeed.
func checkPrivatePermissionProtection(file *os.File) error {
	if file == nil {
		return ErrPrivatePermissionUnavailable
	}
	handle := windows.Handle(file.Fd())
	if err := checkPrivatePermissionVolume(handle); err != nil {
		return err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil || info.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_REPARSE_POINT) != 0 {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	security, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || security == nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	owner, _, err := security.Owner()
	if err != nil || owner == nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil || !owner.Equals(user.User.Sid) {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	control, _, err := security.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	acl, defaulted, err := security.DACL()
	if err != nil || defaulted || acl == nil || acl.AceCount != 1 {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	// x/sys exposes the ACE mask and flags but not a safe SID pointer. The
	// OS-rendered descriptor must contain exactly the checked owner's trustee.
	sddl := security.String()
	ownerIndex, daclIndex := strings.Index(sddl, "O:"), strings.Index(sddl, "D:")
	if ownerIndex != 0 || daclIndex <= 2 {
		return ErrPrivatePermissionUnavailable
	}
	ownerText, daclText := sddl[2:daclIndex], sddl[daclIndex:]
	if strings.Count(daclText, "(A;") != 1 || strings.Count(daclText, ";;;"+ownerText+")") != 1 {
		return ErrPrivatePermissionUnavailable
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(acl, 0, &ace); err != nil || ace == nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags != 0 || ace.Mask != windows.GENERIC_ALL && ace.Mask != windowsFileAllAccess {
		return ErrPrivatePermissionUnavailable
	}
	return nil
}

// Windows needs an explicit owner-only descriptor at creation: chmod cannot
// remove inherited DACL grants, and no sensitive bytes precede verification.
func createCredentialStaging(parent, prefix string, private bool) (*os.File, error) {
	if !private {
		return os.CreateTemp(parent, prefix)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return nil, errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	sid := user.User.Sid.String()
	descriptor, err := windows.SecurityDescriptorFromString("O:" + sid + "D:P(A;;GA;;;" + sid + ")")
	if err != nil {
		return nil, errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	attributes := &windows.SecurityAttributes{SecurityDescriptor: descriptor}
	attributes.Length = uint32(reflect.TypeOf(*attributes).Size())
	for range 32 {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, errors.Join(ErrPrivatePermissionUnavailable, err)
		}
		path := filepath.Join(parent, prefix+hex.EncodeToString(random[:]))
		name, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return nil, errors.Join(ErrPrivatePermissionUnavailable, err)
		}
		handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE|windows.READ_CONTROL|windows.DELETE,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, attributes, windows.CREATE_NEW,
			windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if errors.Is(err, windows.ERROR_FILE_EXISTS) || errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			continue
		}
		if err != nil {
			return nil, errors.Join(ErrPrivatePermissionUnavailable, err)
		}
		file := os.NewFile(uintptr(handle), path)
		if file == nil {
			return nil, errors.Join(ErrPrivatePermissionUnavailable, windows.CloseHandle(handle))
		}
		return file, nil
	}
	return nil, ErrPrivatePermissionUnavailable
}

func retainCredentialStaging(private bool) bool { return private }

func closeCredentialStaging(file *os.File, path string, private, open bool) error {
	if !private {
		var closeErr error
		if open {
			closeErr = file.Close()
		}
		removeErr := os.Remove(path)
		if errors.Is(removeErr, os.ErrNotExist) {
			removeErr = nil
		}
		return errors.Join(closeErr, removeErr)
	}
	// Delete the original staging link by its retained handle, never by a
	// potentially replaced name. Other published hard links remain intact.
	disposition := byte(1)
	removeErr := windows.SetFileInformationByHandle(windows.Handle(file.Fd()), windows.FileDispositionInfo, &disposition, 1)
	return errors.Join(removeErr, file.Close())
}

func openCredentialFile(path string, private bool) (*os.File, error) {
	if !private {
		return os.Open(path)
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.READ_CONTROL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		return nil, errors.Join(ErrPrivatePermissionUnavailable, windows.CloseHandle(handle))
	}
	return file, nil
}

func sameCredentialFile(file *os.File, path string) bool {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	handle, err := windows.CreateFile(name, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return false
	}
	var original, named windows.ByHandleFileInformation
	originalErr := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &original)
	namedErr := windows.GetFileInformationByHandle(handle, &named)
	closeErr := windows.CloseHandle(handle)
	return originalErr == nil && namedErr == nil && closeErr == nil && named.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0 &&
		original.VolumeSerialNumber == named.VolumeSerialNumber && original.FileIndexHigh == named.FileIndexHigh && original.FileIndexLow == named.FileIndexLow
}
