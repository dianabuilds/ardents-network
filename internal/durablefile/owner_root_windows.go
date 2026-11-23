//go:build windows

package durablefile

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// OwnerOnlyRootRefusal carries the caller's exact refusal prose for each
// enforcement failure. The mechanism holds no store identity of its own.
type OwnerOnlyRootRefusal struct {
	OwnerSID string
	RootDACL string
	ReadRoot string
	FileDACL string
}

// EnforceOwnerOnlyRoot applies the owner-only permission floor to one store
// root: a protected DACL granting the current process owner full control and
// nobody else, inherited by future children, plus the same owner-only DACL
// without inheritance on every existing regular file. Any enforcement failure
// refuses; there is no weaker accepting path.
//
// Go synthesizes Windows mode bits from FILE_ATTRIBUTE_READONLY rather than
// the DACL (go1.27.4 src/os/types_windows.go mode()), so permission-bit checks
// cannot observe this floor. Enforcement by mutation is the Windows owner-only
// mechanism; the Unix siblings keep their read-only permission-bit checks.
func EnforceOwnerOnlyRoot(root string, refusal OwnerOnlyRootRefusal) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return errors.New(refusal.OwnerSID)
	}
	if err := setOwnerOnlyDACL(root, user.User.Sid, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT); err != nil {
		return errors.New(refusal.RootDACL)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return errors.New(refusal.ReadRoot)
	}
	for _, entry := range entries {
		if entry.IsDir() || setOwnerOnlyDACL(filepath.Join(root, entry.Name()), user.User.Sid, windows.NO_INHERITANCE) != nil {
			return errors.New(refusal.FileDACL)
		}
	}
	return nil
}

// setOwnerOnlyDACL replaces the target DACL with one explicit grant to the
// owner SID. PROTECTED_DACL_SECURITY_INFORMATION stops inheritance from the
// parent, so a broad parent DACL can never re-enter the store root.
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
