//go:build windows

package portable

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

func validateRuntimeBase(path string) error {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("local runtime base is not a directory")
	}
	return nil
}

func secureOwnedDirectory(path string, _ os.FileInfo) error {
	return setOwnerOnlyDACL(path, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT)
}

func secureAttachment(path string) error { return setOwnerOnlyDACL(path, windows.NO_INHERITANCE) }

func setOwnerOnlyDACL(path string, inheritance uint32) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return errors.New("read current user SID")
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       inheritance,
		Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid)},
	}}, nil)
	if err != nil {
		return err
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		return err
	}
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := descriptor.Owner()
	if err != nil || owner == nil || !owner.Equals(user.User.Sid) {
		return errors.New("local path owner is not the current user")
	}
	dacl, defaulted, err := descriptor.DACL()
	if err != nil || dacl == nil || defaulted || dacl.AceCount == 0 {
		return fmt.Errorf("owner-only DACL did not round-trip: %v", err)
	}
	if strings.Count(descriptor.String(), ";;;"+user.User.Sid.String()+")") != int(dacl.AceCount) {
		return errors.New("local DACL contains another trustee")
	}
	control, _, err := descriptor.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return errors.New("local DACL is not protected")
	}
	return nil
}
