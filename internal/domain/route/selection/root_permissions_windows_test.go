//go:build windows

package selection

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// fileAllAccess is FILE_ALL_ACCESS (winnt.h), the file-specific mapping of
// GENERIC_ALL that Windows stores in the effective ACE of an inheritable
// generic container grant.
const fileAllAccess = 0x001f01ff

// requireOwnerOnlyDACL verifies the physical DACL grants full control to the
// process owner SID and nobody else (package-local copy of the shared
// mechanism's test seam). Windows stores an inheritable GENERIC_ALL container
// grant as the documented pair of an effective ACE with mapped file rights and
// an inherit-only generic ACE (ACE inheritance rules); both grant only the
// owner. SDDL renders well-known SIDs as aliases (for example LA), so trustees
// are compared against the owner's own SDDL rendering, and the owner is bound
// to the process token SID through the security descriptor.
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

// Before #793 the Windows floor checked synthesized mode bits, which report
// 0777 for every writable root, so neither store ever opened. Both stores must
// now open a runner temp root by enforcing the protected owner-only DACL.
func TestSelectionStoresOpenOnWindowsOwnerOnlyRoots(t *testing.T) {
	networkID := [32]byte{1}
	view := ClosedSetView{NetworkID: networkID, Now: time.Unix(1_800_000_000, 0).UTC()}
	entryRoot := ownedRootFixture(t)
	entries, err := openClosedSets(ClosedSetConfig{Root: entryRoot, NetworkID: networkID,
		Current: func() (ClosedSetView, error) { return view, nil }})
	if err != nil {
		t.Fatalf("open closed Entry store on Windows: %v", err)
	}
	// The deferred close releases the exclusive lease even when a DACL check
	// fails, so the runner temp directory can always be cleaned up.
	func() {
		defer func() {
			if cerr := entries.Close(); cerr != nil {
				t.Errorf("close closed Entry store: %v", cerr)
			}
		}()
		requireOwnerOnlyDACL(t, entryRoot, true, true)
		// Store records are created after enforcement and inherit the
		// owner-only grant through the root's inheritable ACE.
		requireOwnerOnlyDACL(t, filepath.Join(entryRoot, closedSetMarker), false, false)
		requireOwnerOnlyDACL(t, filepath.Join(entryRoot, "current"), false, false)
	}()
	reopened, err := openClosedSets(ClosedSetConfig{Root: entryRoot, NetworkID: networkID,
		Current: func() (ClosedSetView, error) { return view, nil }})
	if err != nil {
		t.Fatalf("reopen retained closed Entry store on Windows: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}

	interiorRoot := ownedRootFixture(t)
	interior, err := openInteriorStoreMode(interiorRoot, networkID, 1, closedSetFresh)
	if err != nil {
		t.Fatalf("open Interior store on Windows: %v", err)
	}
	func() {
		defer func() {
			if cerr := interior.close(); cerr != nil {
				t.Errorf("close Interior store: %v", cerr)
			}
		}()
		requireOwnerOnlyDACL(t, interiorRoot, true, true)
		requireOwnerOnlyDACL(t, filepath.Join(interiorRoot, interiorMarker), false, false)
	}()
	reopenedInterior, err := openInteriorStoreMode(interiorRoot, networkID, 1, closedSetEither)
	if err != nil {
		t.Fatalf("reopen retained Interior store on Windows: %v", err)
	}
	if err := reopenedInterior.close(); err != nil {
		t.Fatal(err)
	}
}

func TestSelectionRootRefusesWhenDACLCannotBeEnforced(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := validateRootPermissions(file)
	if err == nil || err.Error() != "read entry state root for DACL enforcement" {
		t.Fatalf("unenforceable root = %v, want read-root refusal", err)
	}
}
