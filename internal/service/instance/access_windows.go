//go:build windows

package instance

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func validateRootAccess(root string, _ os.FileInfo) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return ErrInvalid
	}
	descriptor, err := windows.GetNamedSecurityInfo(root, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || descriptor == nil {
		return ErrInvalid
	}
	owner, _, err := descriptor.Owner()
	if err != nil || owner == nil || !owner.Equals(user.User.Sid) {
		return ErrInvalid
	}
	if err := setOwnerOnlyRootDACL(root, user.User.Sid, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT); err != nil {
		return ErrInvalid
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return ErrInvalid
	}
	for _, entry := range entries {
		if entry.IsDir() || setOwnerOnlyRootDACL(filepath.Join(root, entry.Name()), user.User.Sid, windows.NO_INHERITANCE) != nil {
			return ErrInvalid
		}
	}
	return nil
}

func setOwnerOnlyRootDACL(path string, sid *windows.SID, inheritance uint32) error {
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{AccessPermissions: windows.GENERIC_ALL,
		AccessMode: windows.GRANT_ACCESS, Inheritance: inheritance,
		Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(sid)}}}, nil)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}
