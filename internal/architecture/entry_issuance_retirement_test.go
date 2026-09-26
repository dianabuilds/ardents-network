package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ADR-0096 rejected the closed-alpha Entry issuance/verification candidate
// surface: no accepted C0 command consumes an Initiator-side Invite issuer
// or verifier, so the tracer is removed instead of wired. The internal
// validateInvite classifier stays with its live Import/reopen caller.
func TestRejectedEntryIssuanceSurfaceIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"internal/entry/issue.go",
		"internal/entry/verification_test.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("rejected Entry issuance file still exists: %s", relative)
		}
	}

	verification := string(readProjectFile(t, root, "internal/entry/verification.go"))
	for _, forbidden := range []string{"func Verify(", "MinimumReservation", "Insufficient", "Authorization"} {
		if strings.Contains(verification, forbidden) {
			t.Errorf("verification.go still contains rejected surface %q", forbidden)
		}
	}
	contracts := string(readProjectFile(t, root, "internal/entry/contract.go"))
	for _, forbidden := range []string{"type Authorization struct", "MinimumReservation", "Insufficient", "IssueInput"} {
		if strings.Contains(contracts, forbidden) {
			t.Errorf("contract.go still contains rejected surface %q", forbidden)
		}
	}
}

func TestEntryIssuanceRetirementPreservesInternalVerifier(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	verification := string(readProjectFile(t, root, "internal/entry/verification.go"))
	if !strings.Contains(verification, "func validateInvite(raw []byte, input Verification) (invite, Candidate, Class, error)") {
		t.Error("verification.go lost the retained internal validateInvite classifier")
	}
	for _, retained := range []string{"ConflictingRole", "Expired", "WrongDomain", "Incompatible"} {
		if !strings.Contains(verification, retained) {
			t.Errorf("verification.go lost retained classification %q", retained)
		}
	}
	validation := string(readProjectFile(t, root, "internal/entry/validation.go"))
	if !strings.Contains(validation, "validateInvite(raw, Verification{") {
		t.Error("validation.go lost the live owner.validate wiring into validateInvite")
	}
}
