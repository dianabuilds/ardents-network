package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ADR-0096 rejected the closed-alpha Entry issuance/verification candidate
// surface while retaining the internal validateInvite classifier for its live
// Import/reopen callers. ADR-0106 retired those callers together with the
// whole Invite subsystem, superseding the retention rule: the classifier and
// every issuance/verification file are now absent as well.
func TestRejectedEntryIssuanceSurfaceIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"internal/entry/contract.go",
		"internal/entry/issue.go",
		"internal/entry/validation.go",
		"internal/entry/verification.go",
		"internal/entry/verification_test.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("rejected Entry issuance file still exists: %s", relative)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "internal", "entry"))
	if err != nil {
		t.Fatal(err)
	}
	for _, dirEntry := range entries {
		if dirEntry.IsDir() || !strings.HasSuffix(dirEntry.Name(), ".go") {
			continue
		}
		content := string(readProjectFile(t, root, filepath.Join("internal", "entry", dirEntry.Name())))
		for _, forbidden := range []string{"validateInvite", "func Verify(", "func Issue(",
			"MinimumReservation", "Authorization", "IssueInput", "Verification{"} {
			if strings.Contains(content, forbidden) {
				t.Errorf("internal/entry/%s still contains rejected issuance surface %q", dirEntry.Name(), forbidden)
			}
		}
	}
}
