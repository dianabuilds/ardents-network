//go:build windows

package durablefile

import (
	"errors"
	"os"
	"reflect"

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
	if err != nil || user == nil || user.User.Sid == nil {
		return errors.New(refusal.OwnerSID)
	}
	name, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return errors.New(refusal.RootDACL)
	}
	// MAXIMUM_ALLOWED suppresses SetSecurityInfo's recursive propagation.
	// Existing entries are handled explicitly after the directory is sealed.
	handle, err := windows.CreateFile(name, windows.MAXIMUM_ALLOWED,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
			return errors.New(refusal.RootDACL)
		}
		return errors.Join(errors.New(refusal.RootDACL), err)
	}
	directory := os.NewFile(uintptr(handle), root)
	if directory == nil {
		return errors.Join(errors.New(refusal.ReadRoot), windows.CloseHandle(handle))
	}
	return errors.Join(enforceHeldRoot(directory, handle, user.User.Sid, refusal), directory.Close())
}

func enforceHeldRoot(directory *os.File, handle windows.Handle, owner *windows.SID, refusal OwnerOnlyRootRefusal) error {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return errors.New(refusal.ReadRoot)
	}
	if err := checkWindowsFileOwner(handle, owner); err != nil {
		return errors.Join(errors.New(refusal.RootDACL), err)
	}
	if err := setOwnerOnlyDACL(handle, owner, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT); err != nil {
		return errors.Join(errors.New(refusal.RootDACL), err)
	}
	names, err := directory.Readdirnames(-1)
	if err != nil {
		return errors.New(refusal.ReadRoot)
	}
	for _, name := range names {
		if err := sealRootFile(handle, name, owner); err != nil {
			return errors.Join(errors.New(refusal.FileDACL), err)
		}
	}
	return nil
}

func sealRootFile(root windows.Handle, name string, owner *windows.SID) (resultErr error) {
	childName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return err
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		RootDirectory: root,
		ObjectName:    childName,
		Attributes:    windows.OBJ_DONT_REPARSE,
	}
	attributes.Length = uint32(reflect.TypeOf(attributes).Size())
	var file windows.Handle
	var status windows.IO_STATUS_BLOCK
	if err := windows.NtCreateFile(&file, windows.READ_CONTROL|windows.WRITE_DAC|windows.FILE_READ_ATTRIBUTES,
		&attributes, &status, nil, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		windows.FILE_OPEN, windows.FILE_NON_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT, 0, 0); err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, windows.CloseHandle(file)) }()
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(file, &info); err != nil {
		return err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.NumberOfLinks != 1 {
		return errors.New("entry must be a regular file with exactly one link")
	}
	if err := checkWindowsFileOwner(file, owner); err != nil {
		return err
	}
	return setOwnerOnlyDACL(file, owner, windows.NO_INHERITANCE)
}

func checkWindowsFileOwner(file windows.Handle, owner *windows.SID) error {
	security, err := windows.GetSecurityInfo(file, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return errors.Join(errors.New("query file owner"), err)
	}
	if security == nil {
		return errors.New("file security descriptor is absent")
	}
	actual, _, err := security.Owner()
	if err != nil {
		return errors.Join(errors.New("read file owner"), err)
	}
	if actual == nil || !actual.Equals(owner) {
		return errors.New("file owner differs from process user")
	}
	return nil
}

// setOwnerOnlyDACL replaces the target DACL with one explicit grant to the
// owner SID. PROTECTED_DACL_SECURITY_INFORMATION stops inheritance from the
// parent, so a broad parent DACL can never re-enter the store root.
func setOwnerOnlyDACL(file windows.Handle, sid *windows.SID, inheritance uint32) error {
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
	return windows.SetSecurityInfo(file, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}
