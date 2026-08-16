package architecture

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBlockedEntryLabInterfacesStaySeparate(t *testing.T) {
	t.Parallel()
	assertPackageExports(t, "internal/lab/blockedentry", "Config", "Result", "Run")
	assertPackageExports(t, "internal/lab/blockedverify", "Config", "Result", "Verify")
}

func TestBlockedEntryVerifierImportsNeitherHarnessNorCandidate(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(repositoryRoot(t), "internal", "lab", "blockedverify")
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"internal/lab/blockedentry", "internal/bridge", "internal/camouflage"} {
			if bytes.Contains(raw, []byte(forbidden)) {
				t.Errorf("independent verifier source %s imports %s", entry.Name(), forbidden)
			}
		}
	}
}
