package architecture

import (
	"strings"
	"testing"
)

func TestAlphaCorpusDiagnosticHasNoPersistentFloorAuthority(t *testing.T) {
	root := repositoryRoot(t)
	command := string(readProjectFile(t, root, "cmd/ardents-control/main.go"))
	start := strings.Index(command, "func inspectAlphaCorpus(")
	if start < 0 {
		t.Fatal("cannot isolate inspect-alpha-corpus implementation")
	}
	end := strings.Index(command[start:], "\nfunc ")
	if end < 0 {
		t.Fatal("cannot isolate inspect-alpha-corpus implementation")
	}
	diagnostic := command[start : start+end]
	for _, forbidden := range []string{"state-root", "OpenPersistentFloor", ".Observe("} {
		if strings.Contains(diagnostic, forbidden) {
			t.Errorf("inspect-alpha-corpus retains floor authority %q", forbidden)
		}
	}
	acceptance := string(readProjectFile(t, root, "cmd/ardents-control/alpha_corpus_bundle.go"))
	for _, required := range []string{"func acceptAlphaCorpus(", "alpha.OpenPersistentFloor(", "floor.Observe("} {
		if !strings.Contains(acceptance, required) {
			t.Errorf("accept-alpha-corpus lacks sole floor mutation seam %q", required)
		}
	}
}
