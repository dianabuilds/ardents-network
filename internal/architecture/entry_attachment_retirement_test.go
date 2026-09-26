package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ADR-0095 retired the uncalled Entry attachment execution machinery while
// retaining the durable attempt/contact journal schema; ADR-0106 completed
// the F-08 disposition by deleting the whole Invite subsystem, including
// that schema. Both retirements are asserted as one absence inventory.
func TestRetiredEntryAttachmentMachineryIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"internal/entry/admission_history.go",
		"internal/entry/attempt.go",
		"internal/entry/attachment_lifecycle.go",
		"internal/entry/attachment_lifecycle_test.go",
		"internal/entry/carrier_opener_test.go",
		"internal/entry/contact.go",
		"internal/entry/contract.go",
		"internal/entry/guarded_connection.go",
		"internal/entry/import.go",
		"internal/entry/invite.go",
		"internal/entry/open.go",
		"internal/entry/persistence.go",
		"internal/entry/recipient.go",
		"internal/entry/result_json.go",
		"internal/entry/revalidate.go",
		"internal/entry/state.go",
		"internal/entry/validation.go",
		"internal/entry/verification.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired Entry Invite or attachment file still exists: %s", relative)
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
		for _, forbidden := range []string{"attemptRecord", "contactRecord", "admissionRecord", "durableState",
			"settleReplacements", "retireMember", "ValidateNameOrigin", "CandidateOpener"} {
			if strings.Contains(content, forbidden) {
				t.Errorf("internal/entry/%s still carries retired journal or attachment surface %q", dirEntry.Name(), forbidden)
			}
		}
	}
}

func TestEntryAttachmentRetirementPreservesLiveNameOriginChecks(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	nameOrigin := string(readProjectFile(t, root, "internal/service/connection/name_origin.go"))
	if strings.Contains(nameOrigin, "ValidateNameOrigin") {
		t.Error("Service Connection still declares the retired ValidateNameOrigin leaf")
	}
	for _, retained := range []string{"func ContinuesNameOrigin", "func ValidateRecovery"} {
		if !strings.Contains(nameOrigin, retained) {
			t.Errorf("Service Connection name origin lost retained declaration %q", retained)
		}
	}
}
