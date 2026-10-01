//go:build linux

package installation

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInitialRecoveryJournalRefusesContradictoryPhaseResults(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	journal := replacementTestRoot(t)
	selected := selection{GenerationDigest: digestHex([]byte("generation")), BindingDigest: digestHex([]byte("binding"))}
	intent := transitionIntent{Candidate: selected}
	if err := appendGenerationRecord(journal, "0001.json", selected, "writing-generation", nil); err != nil {
		t.Fatal(err)
	}
	if err := validateInitialRecoveryJournal(journal, intent); err != nil {
		t.Fatal(err)
	}
	for _, example := range []struct {
		name, phase string
		failure     error
	}{
		{"0002.json", "generation-staged", errors.New("original staging refusal")},
		{"fixed-resource-failure.json", "fixed-resources-failed", nil},
		{"selection-failure.json", "selection-failed", nil},
		{"original-transition-failure.json", "successor-transition-failed", nil},
	} {
		if err := appendGenerationRecord(journal, example.name, selected, example.phase, example.failure); err != nil {
			t.Fatal(err)
		}
		if err := validateInitialRecoveryJournal(journal, intent); err == nil {
			t.Fatalf("contradictory phase result accepted: %s", example.name)
		}
		if err := os.Remove(filepath.Join(journal, example.name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := appendGenerationRecord(journal, "0002.json", selected, "generation-write-failed", errors.New("original write refusal")); err != nil {
		t.Fatal(err)
	}
	if err := validateInitialRecoveryJournal(journal, intent); err != nil {
		t.Fatal("valid retained write failure refused", err)
	}
}

func TestInitialIntentRequiresInitialPinAndNoPredecessor(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	root := replacementTestRoot(t)
	request := installationRequestFixture(t)
	request.InstallationRoot = root
	selected := selection{Schema: "ardents-endpoint-installation-selection-v1", GenerationDigest: digestHex([]byte("descriptor"))}
	binding := localBinding{InstallationRoot: root, GenerationDigest: selected.GenerationDigest}
	encoded, err := canonicalJSON(binding)
	if err != nil {
		t.Fatal(err)
	}
	selected.BindingDigest = digestHex(encoded)
	intent := transitionIntent{Schema: "ardents-endpoint-installation-initial-v1", Request: request, Candidate: selected, CandidateBinding: binding}
	write := func(value transitionIntent) {
		t.Helper()
		body, err := canonicalJSON(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "transition.json"), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(intent)
	if _, err := readTransitionIntent(root); err != nil {
		t.Fatal(err)
	}
	missingPin := intent
	missingPin.Request.ManifestSHA256 = ""
	write(missingPin)
	if _, err := readTransitionIntent(root); err == nil {
		t.Fatal("initial intent without original pin provenance passed")
	}
	predecessor := intent
	predecessor.Previous = selected
	write(predecessor)
	if _, err := readTransitionIntent(root); err == nil {
		t.Fatal("initial intent accepted a predecessor")
	}
}
