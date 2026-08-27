//go:build ignore

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

func secureOwnedDirectory(path string, _ os.FileInfo) error {
	return setOwnerOnlyDACL(path, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT)
}

func secureAttachment(path string) error {
	return setOwnerOnlyDACL(path, windows.NO_INHERITANCE)
}

func setOwnerOnlyDACL(path string, inheritance uint32) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return errors.New("read current user SID")
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       inheritance,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
		},
	}}, nil)
	if err != nil {
		return err
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil); err != nil {
		return err
	}
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := descriptor.Owner()
	if err != nil || owner == nil || !owner.Equals(user.User.Sid) {
		return errors.New("path owner is not the current user")
	}
	dacl, defaulted, err := descriptor.DACL()
	if err != nil || dacl == nil || defaulted || dacl.AceCount == 0 {
		return fmt.Errorf("owner-only DACL did not round-trip: err=%v defaulted=%t descriptor=%s", err, defaulted, descriptor.String())
	}
	sid := user.User.Sid.String()
	if strings.Count(descriptor.String(), ";;;"+sid+")") != int(dacl.AceCount) {
		return fmt.Errorf("DACL contains a trustee other than the current user: descriptor=%s", descriptor.String())
	}
	control, _, err := descriptor.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return errors.New("DACL is not protected")
	}
	return nil
}

func acquireOwnerLock(path string) (func() error, bool, error) {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, false, errors.New("owner lock must be a regular file, not a link or another entry type")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, false, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	var overlapped windows.Overlapped
	err = windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		&overlapped,
	)
	if err != nil {
		file.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, true, nil
		}
		return nil, false, err
	}
	return func() error {
		unlockErr := windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &overlapped)
		closeErr := file.Close()
		if unlockErr != nil {
			return unlockErr
		}
		return closeErr
	}, false, nil
}
