//go:build linux

package installation

import (
	"os"
	"path/filepath"
	"testing"
)

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
