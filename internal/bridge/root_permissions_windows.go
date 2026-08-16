package bridge

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func validateOwnerOnlyRoot(root string, _ os.FileInfo) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return errors.New("read Bridge state owner SID")
	}
	if err := setOwnerOnlyDACL(root, user.User.Sid, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT); err != nil {
		return errors.New("bridge state root owner-only DACL could not be enforced")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return errors.New("read bridge state root for DACL enforcement")
	}
	for _, entry := range entries {
		if entry.IsDir() || setOwnerOnlyDACL(filepath.Join(root, entry.Name()), user.User.Sid, windows.NO_INHERITANCE) != nil {
			return errors.New("bridge state file owner-only DACL could not be enforced")
		}
	}
	return nil
}

func setOwnerOnlyDACL(path string, sid *windows.SID, inheritance uint32) error {
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       inheritance,
		Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID,
			TrusteeType: windows.TRUSTEE_IS_USER, TrusteeValue: windows.TrusteeValueFromSID(sid)},
	}}, nil)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}
