//go:build windows

package durablefile

import (
	"errors"
	"os"
	"reflect"
	"strings"

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

// EnforceOwnerOnlyRoot verifies the original directory and entries without
// changing their security descriptors. Existing unsafe objects are refused.
func EnforceOwnerOnlyRoot(root string, refusal OwnerOnlyRootRefusal) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return errors.New(refusal.OwnerSID)
	}
	name, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return errors.New(refusal.RootDACL)
	}
	handle, err := windows.CreateFile(name, windows.READ_CONTROL|windows.FILE_LIST_DIRECTORY|windows.FILE_READ_ATTRIBUTES,
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
	if err := checkOwnerOnlyDACL(handle, owner, true); err != nil {
		return errors.Join(errors.New(refusal.RootDACL), err)
	}
	names, err := directory.Readdirnames(-1)
	if err != nil {
		return errors.New(refusal.ReadRoot)
	}
	for _, name := range names {
		if err := inspectRootFile(handle, name, owner); err != nil {
			return errors.Join(errors.New(refusal.FileDACL), err)
		}
	}
	return nil
}

func inspectRootFile(root windows.Handle, name string, owner *windows.SID) (resultErr error) {
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
	if err := windows.NtCreateFile(&file, windows.READ_CONTROL|windows.FILE_READ_ATTRIBUTES,
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
	return checkOwnerOnlyDACL(file, owner, false)
}

func checkOwnerOnlyDACL(file windows.Handle, owner *windows.SID, directory bool) error {
	security, err := windows.GetSecurityInfo(file, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || security == nil {
		return errors.Join(errors.New("read security descriptor"), err)
	}
	actual, _, err := security.Owner()
	if err != nil || actual == nil || !actual.Equals(owner) {
		return errors.Join(errors.New("file owner differs from process user"), err)
	}
	control, _, err := security.Control()
	if err != nil || directory && control&windows.SE_DACL_PROTECTED == 0 {
		return errors.Join(errors.New("root DACL is not protected"), err)
	}
	acl, defaulted, err := security.DACL()
	if err != nil || defaulted || acl == nil || acl.AceCount == 0 || acl.AceCount > 2 {
		return errors.Join(errors.New("owner-only DACL is absent or invalid"), err)
	}
	sddl := security.String()
	ownerIndex, daclIndex := strings.Index(sddl, "O:"), strings.Index(sddl, "D:")
	if ownerIndex != 0 || daclIndex <= 2 {
		return errors.New("security descriptor has unexpected shape")
	}
	ownerText, daclText := sddl[2:daclIndex], sddl[daclIndex:]
	count := int(acl.AceCount)
	if strings.Count(daclText, "(A;") != count || strings.Count(daclText, ";;;"+ownerText+")") != count {
		return errors.New("DACL grants another principal")
	}
	effective, inherits := false, false
	for i := 0; i < count; i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, uint32(i), &ace); err != nil || ace == nil {
			return errors.Join(errors.New("read DACL entry"), err)
		}
		const fileAllAccess = 0x001f01ff
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Mask != windows.GENERIC_ALL && ace.Mask != fileAllAccess {
			return errors.New("DACL entry is not an owner full-control grant")
		}
		flags := ace.Header.AceFlags
		allowed := uint8(windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE | windows.INHERIT_ONLY_ACE | windows.INHERITED_ACE)
		if flags & ^allowed != 0 {
			return errors.New("DACL entry has unsupported flags")
		}
		if flags&windows.INHERIT_ONLY_ACE == 0 {
			effective = true
		}
		inheritance := flags & (windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE)
		if inheritance == windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE {
			inherits = true
		}
		if !directory && inheritance != 0 {
			return errors.New("file DACL is inheritable")
		}
	}
	if !effective || directory && !inherits {
		return errors.New("DACL does not grant required access")
	}
	return nil
}
