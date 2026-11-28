package custody

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"golang.org/x/sys/windows"
)

// Apply an independently broad DACL to expose a mode-only or inherited-DACL
// implementation. The test owner retains full control for cleanup.
func grantEveryoneForPermissionTest(t *testing.T, path string, inherited bool) {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	everyone, err := windows.CreateWellKnownSid(windows.WinWorldSid)
	if err != nil {
		t.Fatal(err)
	}
	flags := uint32(windows.NO_INHERITANCE)
	if inherited {
		flags = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		{AccessPermissions: windows.GENERIC_ALL, AccessMode: windows.GRANT_ACCESS, Inheritance: flags,
			Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_USER, TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid)}},
		{AccessPermissions: windows.GENERIC_READ, AccessMode: windows.GRANT_ACCESS, Inheritance: flags,
			Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_WELL_KNOWN_GROUP, TrusteeValue: windows.TrusteeValueFromSID(everyone)}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal("set independent broad DACL", err)
	}
}

func TestWindowsPrivatePermissionProtectedCreationAndRetry(t *testing.T) {
	directory := t.TempDir()
	grantEveryoneForPermissionTest(t, directory, true)
	ordinary, err := os.CreateTemp(directory, "unprotected-")
	if err != nil {
		t.Fatal(err)
	}
	if err := checkPrivatePermissionProtection(ordinary); !errors.Is(err, ErrPrivatePermissionUnavailable) {
		t.Fatal("ordinary inherited DACL did not expose the control", err)
	}
	ordinaryPath := ordinary.Name()
	if err := errors.Join(ordinary.Close(), os.Remove(ordinaryPath)); err != nil {
		t.Fatal(err)
	}
	body := bytes.Repeat([]byte{7}, admission.PermissionSize)
	receipt := Receipt{Operation: OperationIssueAdmissionPermission, AdmissionPermission: body}
	path := filepath.Join(directory, "permission")
	if err := receipt.PublishAdmissionPermission(path); err != nil {
		t.Fatal("protected first delivery", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	file, err := openCredentialFile(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(checkPrivatePermissionProtection(file), file.Close()); err != nil {
		t.Fatal("published DACL is not owner-only", err)
	}
	if err := receipt.PublishAdmissionPermission(path); err != nil {
		t.Fatal("exact protected retry", err)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("retry replaced original", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatal("published permission differs", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatal("staging residue", err, entries)
	}
}

func TestWindowsPrivatePermissionRefusesExistingBroadDACL(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "permission")
	body := bytes.Repeat([]byte{9}, admission.PermissionSize)
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	grantEveryoneForPermissionTest(t, path, false)
	before, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	receipt := Receipt{Operation: OperationIssueAdmissionPermission, AdmissionPermission: body}
	if err := receipt.PublishAdmissionPermission(path); !errors.Is(err, ErrPrivatePermissionUnavailable) {
		t.Fatal("unsafe existing permission was accepted", err)
	}
	after, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || before.String() != after.String() {
		t.Fatal("refusal changed the existing DACL", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatal("refusal changed existing bytes", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatal("refusal left staging residue", err, entries)
	}
}
