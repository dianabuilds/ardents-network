//go:build windows

package endpoint

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func secureTransitAcquisitionRoot(root string, _ os.FileInfo) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return errors.New("read transit acquisition owner SID")
	}
	if err := setTransitAcquisitionDACL(root, user.User.Sid, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT); err != nil {
		return errors.New("transit acquisition root owner-only DACL could not be enforced")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || setTransitAcquisitionDACL(filepath.Join(root, entry.Name()), user.User.Sid, windows.NO_INHERITANCE) != nil {
			return errors.New("transit acquisition file owner-only DACL could not be enforced")
		}
	}
	return nil
}

func setTransitAcquisitionDACL(path string, sid *windows.SID, inheritance uint32) error {
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{AccessPermissions: windows.GENERIC_ALL,
		AccessMode: windows.GRANT_ACCESS, Inheritance: inheritance, Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID,
			TrusteeType: windows.TRUSTEE_IS_USER, TrusteeValue: windows.TrusteeValueFromSID(sid)}}}, nil)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}
