package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetiredEntryAttachmentMachineryIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"internal/entry/attempt.go",
		"internal/entry/attachment_lifecycle.go",
		"internal/entry/attachment_lifecycle_test.go",
		"internal/entry/carrier_opener_test.go",
		"internal/entry/contact.go",
		"internal/entry/guarded_connection.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired Entry attachment machinery file still exists: %s", relative)
		}
	}

	contract := string(readProjectFile(t, root, "internal/entry/contract.go"))
	for _, forbidden := range []string{"CandidateOpener", "type Presentation struct", "type Attempt struct",
		"attachments", "acquisitions", "nextAttachment", "cancelLifecycle"} {
		if strings.Contains(contract, forbidden) {
			t.Errorf("Entry contract still declares retired attachment surface %q", forbidden)
		}
	}
	openFile := string(readProjectFile(t, root, "internal/entry/open.go"))
	for _, forbidden := range []string{"settleClosingAttempt", "attachments", "acquisitions", "cancelLifecycle"} {
		if strings.Contains(openFile, forbidden) {
			t.Errorf("Entry open/close still carries retired attachment lifecycle %q", forbidden)
		}
	}
	recipient := string(readProjectFile(t, root, "internal/entry/recipient.go"))
	if strings.Contains(recipient, "func (owner *owner) RecipientCertificate()") {
		t.Error("Entry recipient identity still exports the retired attachment certificate accessor")
	}
	nameOrigin := string(readProjectFile(t, root, "internal/service/connection/name_origin.go"))
	if strings.Contains(nameOrigin, "ValidateNameOrigin") {
		t.Error("Service Connection still declares the retired ValidateNameOrigin leaf")
	}
}

func TestEntryAttachmentRetirementPreservesRetainedDataContracts(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	state := string(readProjectFile(t, root, "internal/entry/state.go"))
	for _, retained := range []string{"type attemptRecord struct", "type contactRecord struct",
		"*attemptRecord", "[]contactRecord", "settleReplacements"} {
		if !strings.Contains(state, retained) {
			t.Errorf("Entry durable state lost retained journal schema member %q", retained)
		}
	}

	openFile := string(readProjectFile(t, root, "internal/entry/open.go"))
	for _, retained := range []string{`"entry-interrupted"`, "retireInvalidVerifiedLocked", "settleReplacements"} {
		if !strings.Contains(openFile, retained) {
			t.Errorf("Entry Open lost the retained legacy-journal recovery %q", retained)
		}
	}

	revalidate := string(readProjectFile(t, root, "internal/entry/revalidate.go"))
	for _, retained := range []string{"func (owner *owner) validRecord", "func (owner *owner) retireInvalidVerifiedLocked"} {
		if !strings.Contains(revalidate, retained) {
			t.Errorf("Entry revalidation lost retained declaration %q", retained)
		}
	}
	if strings.Contains(revalidate, "retireInvalidActiveLocked") {
		t.Error("Entry revalidation still carries the retired pre-carrier sweep")
	}

	importFile := string(readProjectFile(t, root, "internal/entry/import.go"))
	if !strings.Contains(importFile, `next.Attempt != nil && next.Attempt.Terminal == ""`) {
		t.Error("Entry Import lost the retained draining replacement branch over the legacy journal schema")
	}

	persistence := string(readProjectFile(t, root, "internal/entry/persistence.go"))
	if !strings.Contains(persistence, "validAttemptState") {
		t.Error("Entry persistence lost the retained legacy journal validation")
	}

	recipient := string(readProjectFile(t, root, "internal/entry/recipient.go"))
	if !strings.Contains(recipient, "func (owner *owner) RecipientPublicKey()") {
		t.Error("Entry recipient identity lost the retained public accessor")
	}

	nameOrigin := string(readProjectFile(t, root, "internal/service/connection/name_origin.go"))
	for _, retained := range []string{"func ContinuesNameOrigin", "func ValidateRecovery"} {
		if !strings.Contains(nameOrigin, retained) {
			t.Errorf("Service Connection name origin lost retained declaration %q", retained)
		}
	}
}
