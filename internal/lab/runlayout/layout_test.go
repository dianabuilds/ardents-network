package runlayout

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLayoutDerivesAndRevalidatesOwnedPaths(t *testing.T) {
	t.Parallel()
	temporaryRoot := t.TempDir()
	runID := "20260810T120000Z-42"
	sessionRoot := filepath.Join(temporaryRoot, SessionPrefix+runID)
	repositoryRoot := filepath.Join(temporaryRoot, "repository")
	for _, directory := range []string{sessionRoot, repositoryRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	layout, err := New(sessionRoot, repositoryRoot, temporaryRoot, runID)
	if err != nil {
		t.Fatal(err)
	}
	gotID, gotRepository, runDirectory, evidenceDirectory, err := layout.OwnedPaths(false, false)
	if err != nil {
		t.Fatal(err)
	}
	if gotID != runID || gotRepository != repositoryRoot {
		t.Fatalf("identity = %q %q", gotID, gotRepository)
	}
	if runDirectory != filepath.Join(sessionRoot, RunPrefix+runID) {
		t.Fatalf("run directory = %q", runDirectory)
	}
	if evidenceDirectory != filepath.Join(sessionRoot, EvidencePrefix+runID) {
		t.Fatalf("evidence directory = %q", evidenceDirectory)
	}
	if err := os.MkdirAll(runDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := layout.OwnedPaths(true, false); err != nil {
		t.Fatalf("revalidate existing owned directory: %v", err)
	}
}

func TestLayoutRejectsUnownedIdentityAndPaths(t *testing.T) {
	t.Parallel()
	temporaryRoot := t.TempDir()
	repositoryRoot := filepath.Join(temporaryRoot, "repository")
	if err := os.MkdirAll(repositoryRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		runID   string
		session string
	}{
		{name: "unsafe ID", runID: "../escape", session: filepath.Join(temporaryRoot, SessionPrefix+"escape")},
		{name: "wrong session name", runID: "safe", session: filepath.Join(temporaryRoot, "unowned.safe")},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.MkdirAll(test.session, 0o700); err != nil {
				t.Fatal(err)
			}
			if _, err := New(test.session, repositoryRoot, temporaryRoot, test.runID); err == nil {
				t.Fatal("New accepted an unowned layout")
			}
		})
	}
}
