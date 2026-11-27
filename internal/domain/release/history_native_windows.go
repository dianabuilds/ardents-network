package release

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
)

const historyPlatformSupported = true

const (
	historyFilesystem = "NTFS"
	fileAllAccess     = 0x001f01ff
	// Windows uses FILE_APPEND_DATA as FILE_ADD_SUBDIRECTORY on directories.
	addSubdirectory = windows.FILE_APPEND_DATA
)

// historyRoot holds the original root and its parent without delete sharing.
// It prevents a replacement of either named directory while the store is open.
type historyRoot struct {
	parent windows.Handle
	root   *os.File
}

type historyLease struct{ handle windows.Handle }

// openHistoryRoot creates a new root with its protected descriptor atomically,
// or opens an existing root without changing its ACL. The returned handles are
// retained through the original lease and all recovery work.
func openHistoryRoot(root string, retained bool) (*historyRoot, error) {
	parentPath := filepath.Dir(root)
	if parentPath == root {
		return nil, ErrTrustUnavailable
	}
	if !retained {
		if err := prepareHistoryParent(parentPath); err != nil {
			return nil, err
		}
	}
	original, originalErr := os.Lstat(root)
	create := !retained && errors.Is(originalErr, os.ErrNotExist)
	if originalErr != nil && !create {
		return nil, errors.Join(ErrTrustUnavailable, originalErr)
	}
	parentAccess := uint32(windows.GENERIC_READ)
	if create {
		parentAccess |= addSubdirectory
	}
	parent, err := openHistoryDirectory(parentPath, parentAccess)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*historyRoot, error) {
		return nil, errors.Join(ErrTrustUnavailable, err, windows.CloseHandle(parent))
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fail(err)
	}
	sid := user.User.Sid.String()
	descriptor, err := windows.SecurityDescriptorFromString("O:" + sid + "D:P(A;OICI;GA;;;" + sid + ")")
	if err != nil {
		return fail(err)
	}
	leaf, err := windows.NewNTUnicodeString(filepath.Base(root))
	if err != nil {
		return fail(err)
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		RootDirectory:      parent,
		ObjectName:         leaf,
		Attributes:         windows.OBJ_CASE_INSENSITIVE,
		SecurityDescriptor: descriptor,
	}
	// OBJECT_ATTRIBUTES changes size with the Windows architecture.
	attributes.Length = uint32(reflect.TypeOf(attributes).Size())
	disposition := uint32(windows.FILE_OPEN)
	if create {
		disposition = windows.FILE_CREATE
	}
	var handle windows.Handle
	var status windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&handle, windows.FILE_LIST_DIRECTORY|windows.FILE_READ_ATTRIBUTES|windows.READ_CONTROL|windows.SYNCHRONIZE,
		&attributes, &status, nil, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		disposition, windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	if err != nil {
		return fail(err)
	}
	file := os.NewFile(uintptr(handle), root)
	if file == nil {
		return fail(windows.CloseHandle(handle))
	}
	if err := checkHistoryObjectHandle(handle, true, true); err != nil {
		return nil, errors.Join(fmt.Errorf("release: root security: %w", err), file.Close(), windows.CloseHandle(parent))
	}
	opened, openErr := file.Stat()
	named, nameErr := os.Lstat(root)
	if openErr != nil || nameErr != nil || !os.SameFile(opened, named) ||
		!create && !os.SameFile(original, opened) {
		return nil, errors.Join(fmt.Errorf("release: root identity: %w", ErrTrustUnavailable), openErr, nameErr, file.Close(), windows.CloseHandle(parent))
	}
	return &historyRoot{parent: parent, root: file}, nil
}

// Check an existing ancestor before creating any missing parent directories.
// A filesystem that cannot preserve ACLs therefore refuses before creation.
func prepareHistoryParent(path string) error {
	ancestor := path
	for {
		info, err := os.Lstat(ancestor)
		if err == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return ErrTrustUnavailable
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrTrustUnavailable, err)
		}
		next := filepath.Dir(ancestor)
		if next == ancestor {
			return ErrTrustUnavailable
		}
		ancestor = next
	}
	handle, err := openHistoryDirectory(ancestor, windows.GENERIC_READ)
	if err != nil {
		return err
	}
	if err := windows.CloseHandle(handle); err != nil {
		return errors.Join(ErrTrustUnavailable, err)
	}
	return os.MkdirAll(path, 0700)
}

func openHistoryDirectory(path string, access uint32) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, errors.Join(ErrTrustUnavailable, err)
	}
	handle, err := windows.CreateFile(name, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return windows.InvalidHandle, errors.Join(ErrTrustUnavailable, err)
	}
	var info windows.ByHandleFileInformation
	infoErr := windows.GetFileInformationByHandle(handle, &info)
	if infoErr != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return windows.InvalidHandle, errors.Join(ErrTrustUnavailable, infoErr, windows.CloseHandle(handle))
	}
	if err := checkHistoryVolume(handle); err != nil {
		return windows.InvalidHandle, errors.Join(err, windows.CloseHandle(handle))
	}
	return handle, nil
}

func (root *historyRoot) release() error {
	if root == nil || root.root == nil {
		return nil
	}
	err := errors.Join(root.root.Close(), windows.CloseHandle(root.parent))
	root.root = nil
	return err
}

func checkHistoryVolume(handle windows.Handle) error {
	var flags uint32
	var filesystem [32]uint16
	if err := windows.GetVolumeInformationByHandle(handle, nil, 0, nil, nil, &flags, &filesystem[0], uint32(len(filesystem))); err != nil {
		return errors.Join(ErrTrustUnavailable, err)
	}
	if flags&windows.FILE_PERSISTENT_ACLS == 0 || windows.UTF16ToString(filesystem[:]) != historyFilesystem {
		return fmt.Errorf("release: unsupported history filesystem %q: %w", windows.UTF16ToString(filesystem[:]), ErrTrustUnavailable)
	}
	return nil
}

// New children must name the user explicitly: elevated Windows tokens can
// otherwise choose the Administrators group as the default owner, even when
// the parent has an owner-only inherited DACL. Existing objects are never reset.
func historyCreationSecurity(directory bool) (*windows.SecurityAttributes, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	sid := user.User.Sid.String()
	flags := ""
	if directory {
		flags = "OICI"
	}
	descriptor, err := windows.SecurityDescriptorFromString("O:" + sid + "D:P(A;" + flags + ";GA;;;" + sid + ")")
	if err != nil {
		return nil, err
	}
	attributes := &windows.SecurityAttributes{SecurityDescriptor: descriptor}
	attributes.Length = uint32(reflect.TypeOf(*attributes).Size())
	return attributes, nil
}

func createHistoryFile(path string) (*os.File, error) {
	attributes, err := historyCreationSecurity(false)
	if err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE|windows.READ_CONTROL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, attributes, windows.CREATE_NEW,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		return nil, errors.Join(ErrTrustUnavailable, windows.CloseHandle(handle))
	}
	return file, nil
}

func createHistoryDirectory(path string) error {
	attributes, err := historyCreationSecurity(true)
	if err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	if err := windows.CreateDirectory(name, attributes); err != nil {
		return &os.PathError{Op: "mkdir", Path: path, Err: err}
	}
	return nil
}

// The persisted recovery grammar accepts decimal suffixes of at most ten
// digits. Exclusive creation resolves collisions without exposing a file or
// directory under a default-owner descriptor.
func historyTempName(parent, prefix string) (string, error) {
	var random [4]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return filepath.Join(parent, prefix+strconv.FormatUint(uint64(binary.BigEndian.Uint32(random[:])), 10)), nil
}

func createHistoryTempDirectory(parent, prefix string) (string, error) {
	for attempt := 0; attempt < 100; attempt++ {
		path, err := historyTempName(parent, prefix)
		if err != nil {
			return "", err
		}
		if err = createHistoryDirectory(path); err == nil {
			return path, nil
		} else if !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	return "", errors.New("release: cannot create unique staging directory")
}

func createHistoryTempFile(parent, prefix string) (*os.File, error) {
	for attempt := 0; attempt < 100; attempt++ {
		path, err := historyTempName(parent, prefix)
		if err != nil {
			return nil, err
		}
		file, err := createHistoryFile(path)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	return nil, errors.New("release: cannot create unique staged pointer")
}

// checkHistoryObject rechecks the opened file, not a path-derived mode bit.
// Windows permits bypassing directory traversal checks, so each existing child
// must independently retain the same owner-only floor before it is read.
func checkHistoryObject(file *os.File, directory bool) error {
	return checkHistoryObjectHandle(windows.Handle(file.Fd()), directory, false)
}

func checkHistoryObjectHandle(handle windows.Handle, directory, protected bool) error {
	if err := checkHistoryVolume(handle); err != nil {
		return err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return errors.Join(ErrTrustUnavailable, err)
	}
	isDirectory := info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
	if isDirectory != directory || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		!directory && info.NumberOfLinks != 1 {
		return fmt.Errorf("release: history object kind or links: %w", ErrTrustUnavailable)
	}
	security, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || security == nil {
		return errors.Join(ErrTrustUnavailable, err)
	}
	owner, _, err := security.Owner()
	if err != nil || owner == nil {
		return errors.Join(ErrTrustUnavailable, err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || !owner.Equals(user.User.Sid) {
		return errors.Join(fmt.Errorf("release: history owner: %w", ErrTrustUnavailable), err)
	}
	control, _, err := security.Control()
	if err != nil || protected && control&windows.SE_DACL_PROTECTED == 0 {
		return errors.Join(fmt.Errorf("release: history DACL protection: %w", ErrTrustUnavailable), err)
	}
	acl, defaulted, err := security.DACL()
	if err != nil || defaulted || acl == nil || acl.AceCount == 0 || acl.AceCount > 2 {
		return errors.Join(fmt.Errorf("release: history DACL shape: %w", ErrTrustUnavailable), err)
	}
	// x/sys exposes ACE headers and masks but not a safe pointer to their SID.
	// Compare each trustee with the descriptor's owner in the OS-rendered SDDL;
	// reject unexpected syntax, unknown ACE forms and every additional trustee.
	sddl := security.String()
	ownerIndex, daclIndex := strings.Index(sddl, "O:"), strings.Index(sddl, "D:")
	if ownerIndex != 0 || daclIndex <= ownerIndex+2 {
		return fmt.Errorf("release: history security descriptor shape: %w", ErrTrustUnavailable)
	}
	ownerText, daclText := sddl[ownerIndex+2:daclIndex], sddl[daclIndex:]
	count := int(acl.AceCount)
	if strings.Count(daclText, "(A;") != count || strings.Count(daclText, ";;;"+ownerText+")") != count {
		return fmt.Errorf("release: history DACL trustees: %w", ErrTrustUnavailable)
	}
	effective, propagates := false, false
	for i := 0; i < count; i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, uint32(i), &ace); err != nil || ace == nil {
			return errors.Join(ErrTrustUnavailable, err)
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE ||
			ace.Mask != windows.GENERIC_ALL && ace.Mask != fileAllAccess {
			return fmt.Errorf("release: history ACE type %d mask %#x: %w", ace.Header.AceType, ace.Mask, ErrTrustUnavailable)
		}
		flags := ace.Header.AceFlags
		allowed := uint8(windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE | windows.INHERIT_ONLY_ACE | windows.INHERITED_ACE)
		if flags & ^allowed != 0 {
			return fmt.Errorf("release: history ACE flags %#x: %w", flags, ErrTrustUnavailable)
		}
		if flags&windows.INHERIT_ONLY_ACE == 0 {
			effective = true
		}
		inherit := flags & (windows.CONTAINER_INHERIT_ACE | windows.OBJECT_INHERIT_ACE)
		if inherit == windows.CONTAINER_INHERIT_ACE|windows.OBJECT_INHERIT_ACE {
			propagates = true
		}
		if !directory && inherit != 0 {
			return fmt.Errorf("release: file has inheritable ACE: %w", ErrTrustUnavailable)
		}
	}
	if !effective || directory && !propagates {
		return fmt.Errorf("release: history DACL lacks effective or inherited access: %w", ErrTrustUnavailable)
	}
	return nil
}

// check reobserves the original pinned root before opening any history files.
func (root *historyRoot) check(named os.FileInfo) error {
	opened, err := root.root.Stat()
	if err != nil || !os.SameFile(opened, named) {
		return errors.Join(ErrTrustUnavailable, err)
	}
	return checkHistoryObjectHandle(windows.Handle(root.root.Fd()), true, true)
}

func acquireHistoryLease(root string) (historyLease, error) {
	name, err := windows.UTF16PtrFromString(filepath.Join(root, floorStoreLockName))
	if err != nil {
		return historyLease{}, err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return historyLease{}, err
	}
	if err := checkHistoryObjectHandle(h, false, false); err != nil {
		return historyLease{}, errors.Join(err, windows.CloseHandle(h))
	}
	return historyLease{h}, nil
}

func (l *historyLease) release() error {
	if l.handle == windows.InvalidHandle {
		return nil
	}
	err := windows.CloseHandle(l.handle)
	l.handle = windows.InvalidHandle
	return err
}

func durableRename(from, to string) error {
	a, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	b, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(a, b, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

func syncDirectory(path string) error {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	if err := checkHistoryObjectHandle(h, true, false); err != nil {
		return errors.Join(err, windows.CloseHandle(h))
	}
	return errors.Join(windows.FlushFileBuffers(h), windows.CloseHandle(h))
}
