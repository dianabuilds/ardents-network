//go:build windows

package durablefile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

var testRootRefusal = OwnerOnlyRootRefusal{
	OwnerSID: "read test owner SID",
	RootDACL: "test root owner-only DACL could not be enforced",
	ReadRoot: "read test root for DACL enforcement",
	FileDACL: "test file owner-only DACL could not be enforced",
}

// fileAllAccess is FILE_ALL_ACCESS (winnt.h), the file-specific mapping of
// GENERIC_ALL that Windows stores in the effective ACE of an inheritable
// generic container grant.
const fileAllAccess = 0x001f01ff

// requireOwnerOnlyDACL verifies the physical DACL grants full control to the
// process owner SID and nobody else. Windows stores an inheritable GENERIC_ALL
// container grant as the documented pair of an effective ACE with mapped file
// rights and an inherit-only generic ACE (ACE inheritance rules); both grant
// only the owner. SDDL renders well-known SIDs as aliases (for example LA), so
// trustees are compared against the owner's own SDDL rendering, and the owner
// is bound to the process token SID through the security descriptor.
func requireOwnerOnlyDACL(t *testing.T, path string, inheritable, protected bool) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatalf("read security descriptor of %s: %v", path, err)
	}
	owner, _, err := sd.Owner()
	if err != nil {
		t.Fatalf("read owner of %s: %v", path, err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if !owner.Equals(user.User.Sid) {
		t.Fatalf("%s is not owned by the process owner", path)
	}
	acl, defaulted, err := sd.DACL()
	if err != nil || defaulted || acl == nil {
		t.Fatalf("DACL of %s absent or defaulted: %v", path, err)
	}
	sddl := sd.String()
	if !strings.Contains(sddl, "O:") || !strings.Contains(sddl, "D:") {
		t.Fatalf("SDDL of %s lacks owner or DACL: %q", path, sddl)
	}
	ownerSection := sddl[strings.Index(sddl, "O:")+2 : strings.Index(sddl, "D:")]
	daclSection := sddl[strings.Index(sddl, "D:"):]
	if protected && !strings.HasPrefix(daclSection, "D:P") {
		t.Fatalf("DACL of %s is not protected: %s", path, sddl)
	}
	aceCount := int(acl.AceCount)
	if aceCount < 1 || aceCount > 2 || !inheritable && aceCount != 1 ||
		strings.Count(daclSection, "(A;") != aceCount ||
		strings.Count(daclSection, ";;;"+ownerSection+")") != aceCount {
		t.Fatalf("DACL of %s must grant only the owner: %s", path, sddl)
	}
	effective, propagates := false, false
	for i := 0; i < aceCount; i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, uint32(i), &ace); err != nil {
			t.Fatalf("read ACE %d of %s: %v", i, path, err)
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE ||
			ace.Mask != windows.GENERIC_ALL && ace.Mask != fileAllAccess {
			t.Fatalf("ACE %d of %s is type %d mask %x, want an owner full-control grant: %s",
				i, path, ace.Header.AceType, ace.Mask, sddl)
		}
		flags := ace.Header.AceFlags
		if flags&windows.INHERIT_ONLY_ACE == 0 {
			effective = true
		}
		inheritBits := flags & (windows.CONTAINER_INHERIT_ACE | windows.OBJECT_INHERIT_ACE)
		if inheritBits == windows.CONTAINER_INHERIT_ACE|windows.OBJECT_INHERIT_ACE {
			propagates = true
		}
		if !inheritable && inheritBits != 0 {
			t.Fatalf("file %s carries an inheritable ACE: %s", path, sddl)
		}
	}
	if !effective {
		t.Fatalf("DACL of %s has no effective owner grant: %s", path, sddl)
	}
	if inheritable && !propagates {
		t.Fatalf("DACL of %s does not propagate the owner grant: %s", path, sddl)
	}
}

func TestEnforceOwnerOnlyRootGrantsOnlyOwner(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "record")
	if err := os.WriteFile(existing, []byte("record"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnforceOwnerOnlyRoot(root, testRootRefusal); err != nil {
		t.Fatalf("enforce owner-only root: %v", err)
	}
	requireOwnerOnlyDACL(t, root, true, true)
	requireOwnerOnlyDACL(t, existing, false, true)
	// A file created after enforcement inherits the owner-only grant through
	// the root's inheritable ACE, never the runner temp directory's DACL.
	inherited := filepath.Join(root, "later")
	if err := os.WriteFile(inherited, []byte("later"), 0o600); err != nil {
		t.Fatal(err)
	}
	requireOwnerOnlyDACL(t, inherited, false, false)
}

func TestEnforceOwnerOnlyRootReplacesBroadGrant(t *testing.T) {
	root := t.TempDir()
	everyone, err := windows.CreateWellKnownSid(windows.WinWorldSid)
	if err != nil {
		t.Fatal(err)
	}
	broad, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID,
			TrusteeType: windows.TRUSTEE_IS_USER, TrusteeValue: windows.TrusteeValueFromSID(everyone)},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(root, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, broad, nil); err != nil {
		t.Fatal(err)
	}
	if err := EnforceOwnerOnlyRoot(root, testRootRefusal); err != nil {
		t.Fatalf("enforce owner-only root: %v", err)
	}
	requireOwnerOnlyDACL(t, root, true, true)
}

func TestEnforceOwnerOnlyRootRefusesUnenforceableRoot(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	if err := EnforceOwnerOnlyRoot(missing, testRootRefusal); err == nil || err.Error() != testRootRefusal.RootDACL {
		t.Fatalf("missing root = %v, want root DACL refusal", err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnforceOwnerOnlyRoot(file, testRootRefusal); err == nil || err.Error() != testRootRefusal.ReadRoot {
		t.Fatalf("file root = %v, want read-root refusal", err)
	}
}

func TestEnforceOwnerOnlyRootRefusesExternalHardLink(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(outside, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, filepath.Join(root, "record")); err != nil {
		t.Fatal(err)
	}
	before, err := windows.GetNamedSecurityInfo(outside, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	if err := EnforceOwnerOnlyRoot(root, testRootRefusal); err == nil {
		t.Error("external hard link was not refused")
	}
	after, err := windows.GetNamedSecurityInfo(outside, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	if before.String() != after.String() {
		t.Error("external target DACL changed")
	}
}

func TestEnforceOwnerOnlyRootDoesNotPropagateToExistingDirectory(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "unexpected-directory")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	before, err := windows.GetNamedSecurityInfo(child, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := before.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED != 0 {
		t.Fatal("control must be subject to inheritance", err)
	}
	if err := EnforceOwnerOnlyRoot(root, testRootRefusal); err == nil {
		t.Fatal("unexpected directory was accepted")
	}
	after, err := windows.GetNamedSecurityInfo(child, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	if before.String() != after.String() {
		t.Error("root write propagated to an unvalidated child")
	}
}

func TestEnforceOwnerOnlyRootRefusesRedirectedEntry(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside")
	if err := os.WriteFile(outside, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "record")); err != nil {
		t.Fatal(err)
	}
	before, err := windows.GetNamedSecurityInfo(outside, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	if err := EnforceOwnerOnlyRoot(root, testRootRefusal); err == nil {
		t.Fatal("redirected entry was accepted")
	}
	after, err := windows.GetNamedSecurityInfo(outside, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	if before.String() != after.String() {
		t.Error("redirected target DACL changed")
	}
}
