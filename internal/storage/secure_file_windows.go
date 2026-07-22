//go:build windows

package storage

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func validatePrivateFile(path string, _ os.FileInfo) error {
	return protectPrivatePath(path, false)
}

func validateStrictPrivateFile(path string, _ os.FileInfo) error {
	return validatePrivateACL(path, false)
}

func validatePrivateDirectory(path string, _ os.FileInfo) error {
	return validatePrivateACL(path, true)
}

func validatePrivateACL(path string, container bool) error {
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	control, _, err := descriptor.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return fmt.Errorf("private state ACL is not protected")
	}
	dacl, _, err := descriptor.DACL()
	expectedCount := uint16(2)
	if container {
		expectedCount = 4
	}
	if err != nil || dacl == nil || dacl.AceCount != expectedCount {
		return fmt.Errorf("private state ACL has unexpected trustees")
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	system, err := windows.StringToSid("S-1-5-18")
	if err != nil {
		return err
	}
	const fileAllAccess windows.ACCESS_MASK = windows.STANDARD_RIGHTS_REQUIRED | windows.SYNCHRONIZE | 0x1ff
	const inheritFlags = uint8(windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE | windows.INHERIT_ONLY_ACE)
	var userDirect, userInherited, systemDirect, systemInherited bool
	for index := uint32(0); index < uint32(dacl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, index, &ace); err != nil {
			return err
		}
		if ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return fmt.Errorf("private state ACL has unexpected access")
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		direct := ace.Header.AceFlags == 0 && ace.Mask == fileAllAccess
		inherited := container && ace.Header.AceFlags == inheritFlags && ace.Mask == windows.GENERIC_ALL
		if !direct && !inherited {
			return fmt.Errorf("private state ACL has unexpected access")
		}
		switch {
		case sid.Equals(user.User.Sid) && direct && !userDirect:
			userDirect = true
		case sid.Equals(user.User.Sid) && inherited && !userInherited:
			userInherited = true
		case sid.Equals(system) && direct && !systemDirect:
			systemDirect = true
		case sid.Equals(system) && inherited && !systemInherited:
			systemInherited = true
		default:
			return fmt.Errorf("private state ACL has unexpected trustees")
		}
	}
	if !userDirect || !systemDirect || container && (!userInherited || !systemInherited) {
		return fmt.Errorf("private state ACL has unexpected trustees")
	}
	return nil
}

func protectPrivatePath(path string, container bool) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	system, err := windows.StringToSid("S-1-5-18")
	if err != nil {
		return err
	}
	inheritance := uint32(windows.NO_INHERITANCE)
	if container {
		inheritance = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	entries := []windows.EXPLICIT_ACCESS{
		privateAccessEntry(user.User.Sid, inheritance),
		privateAccessEntry(system, inheritance),
	}
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil,
	)
}

func privateAccessEntry(sid *windows.SID, inheritance uint32) windows.EXPLICIT_ACCESS {
	return windows.EXPLICIT_ACCESS{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       inheritance,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
}

func replacePrivateFile(source, target string) error {
	sourcePtr, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(
		sourcePtr,
		targetPtr,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	)
}

func publishPrivateFileNoReplace(source, target string) error {
	sourcePtr, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(sourcePtr, targetPtr, windows.MOVEFILE_WRITE_THROUGH)
}
