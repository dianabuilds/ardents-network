package release

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// Replace only the selected object's DACL; restore it even when an assertion
// fails so TempDir cleanup never depends on an intentionally hostile ACL.
func allowEveryoneForHistoryTest(t *testing.T, path string) func() {
	t.Helper()
	original, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	previous, _, err := original.DACL()
	if err != nil || previous == nil {
		t.Fatal("original DACL unavailable", err)
	}
	everyone, err := windows.CreateWellKnownSid(windows.WinWorldSid)
	if err != nil {
		t.Fatal(err)
	}
	broad, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID,
			TrusteeType: windows.TRUSTEE_IS_WELL_KNOWN_GROUP, TrusteeValue: windows.TrusteeValueFromSID(everyone)},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, broad, nil); err != nil {
		t.Fatal("set unprotected test DACL", err)
	}
	restore := func() {
		if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
			windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, previous, nil); err != nil {
			t.Error("restore test DACL", err)
		}
	}
	return restore
}

func TestWindowsRetainedHistoryRefusesBroadDACLBeforeRecovery(t *testing.T) {
	for _, target := range []string{"root", "lease", "current"} {
		t.Run(target, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "history")
			verifier, err := Open(root)
			if err != nil {
				t.Fatal("protected first history", err)
			}
			if decision := verifier.Evaluate(t.Context(), publicVector(t)); decision.Outcome != OutcomeReleaseAccepted {
				t.Fatal("signed history", decision.Err())
			}
			if err := verifier.Close(); err != nil {
				t.Fatal(err)
			}
			current := filepath.Join(root, "current")
			before, err := os.ReadFile(current)
			if err != nil {
				t.Fatal(err)
			}
			residue := filepath.Join(root, ".current-12345")
			if err := writeSyncedFile(residue, []byte("012345abcdef")); err != nil {
				t.Fatal(err)
			}
			path := root
			if target == "lease" {
				path = filepath.Join(root, floorStoreLockName)
			} else if target == "current" {
				path = current
			}
			restore := allowEveryoneForHistoryTest(t, path)
			t.Cleanup(restore)
			for _, retained := range []bool{true, false} {
				var reopened *Verifier
				if retained {
					reopened, err = OpenRetained(root)
				} else {
					reopened, err = Open(root)
				}
				if reopened != nil {
					_ = reopened.Close()
				}
				if !errors.Is(err, ErrTrustUnavailable) || reopened != nil {
					t.Fatalf("unsafe %s accepted (retained=%v): %v", target, retained, err)
				}
				after, readErr := os.ReadFile(current)
				if readErr != nil || !bytes.Equal(before, after) {
					t.Fatal("refusal changed committed history", readErr)
				}
				if _, err := os.Stat(residue); err != nil {
					t.Fatal("refusal removed writer residue", err)
				}
			}
			restore()
			reopened, err := OpenRetained(root)
			if err != nil {
				t.Fatal("protected retained history refused", err)
			}
			if err := reopened.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(residue); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("protected recovery did not remove the original residue", err)
			}
		})
	}
}

func TestWindowsRetainedHistoryRefusesForeignOwnerBeforeRecovery(t *testing.T) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	groups, err := windows.GetCurrentProcessToken().GetTokenGroups()
	if err != nil {
		t.Fatal(err)
	}
	var foreignOwner *windows.SID
	for _, group := range groups.AllGroups() {
		if group.Attributes&windows.SE_GROUP_OWNER != 0 && !group.Sid.Equals(user.User.Sid) {
			foreignOwner, err = group.Sid.Copy()
			if err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if foreignOwner == nil {
		t.Fatal("invalid environment: token needs an assignable non-user owner SID")
	}
	for _, target := range []string{"root", "lease"} {
		t.Run(target, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "history")
			verifier, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			if decision := verifier.Evaluate(t.Context(), publicVector(t)); decision.Outcome != OutcomeReleaseAccepted {
				t.Fatal(decision.Err())
			}
			if err := verifier.Close(); err != nil {
				t.Fatal(err)
			}
			current := filepath.Join(root, "current")
			before, err := os.ReadFile(current)
			if err != nil {
				t.Fatal(err)
			}
			residue := filepath.Join(root, ".current-12345")
			if err := writeSyncedFile(residue, []byte("012345abcdef")); err != nil {
				t.Fatal(err)
			}
			path := root
			if target == "lease" {
				path = filepath.Join(root, floorStoreLockName)
			}
			if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
				windows.OWNER_SECURITY_INFORMATION, foreignOwner, nil, nil, nil); err != nil {
				t.Fatal("invalid environment: could not set alternate owner", err)
			}
			restore := func() {
				if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
					windows.OWNER_SECURITY_INFORMATION, user.User.Sid, nil, nil, nil); err != nil {
					t.Error("restore original owner", err)
				}
			}
			t.Cleanup(restore)
			reopened, err := OpenRetained(root)
			if reopened != nil {
				_ = reopened.Close()
			}
			if !errors.Is(err, ErrTrustUnavailable) || reopened != nil {
				t.Fatal("foreign owner accepted", err)
			}
			after, err := os.ReadFile(current)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("owner refusal changed committed history", err)
			}
			if _, err := os.Stat(residue); err != nil {
				t.Fatal("owner refusal removed writer residue", err)
			}
			restore()
			reopened, err = OpenRetained(root)
			if err != nil {
				t.Fatal("original owner could not reopen", err)
			}
			if err := reopened.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
