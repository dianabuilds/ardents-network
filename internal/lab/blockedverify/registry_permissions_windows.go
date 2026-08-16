//go:build windows

package blockedverify

import (
	"errors"
	"io/fs"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func protectRegistryTree(root string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return errors.New("read replay-registry owner SID")
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.Type()&fs.ModeSymlink != 0 {
			return errors.Join(walkErr, errors.New("replay registry is unavailable or aliased"))
		}
		descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
			windows.OWNER_SECURITY_INFORMATION)
		if err != nil || descriptor == nil {
			return errors.Join(err, errors.New("replay registry owner cannot be read"))
		}
		owner, _, err := descriptor.Owner()
		if err != nil || owner == nil || !owner.Equals(user.User.Sid) {
			return errors.Join(err, errors.New("replay registry has a foreign owner"))
		}
		inheritance := uint32(windows.NO_INHERITANCE)
		if entry.IsDir() {
			inheritance = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
		}
		acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
			AccessPermissions: windows.GENERIC_ALL, AccessMode: windows.GRANT_ACCESS, Inheritance: inheritance,
			Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID,
				TrusteeType: windows.TRUSTEE_IS_USER, TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid)},
		}}, nil)
		if err != nil {
			return err
		}
		return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
			windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
	})
}
