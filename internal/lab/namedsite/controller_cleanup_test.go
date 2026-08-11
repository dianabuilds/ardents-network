package namedsite

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/lab/runlayout"
)

func TestCleanupGateCPreparationRemovesPartialOwnedLayout(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	runID := "gatec-partial-" + time.Now().UTC().Format("20060102t150405.000000000")
	session := filepath.Join(os.TempDir(), runlayout.SessionPrefix+runID)
	if err := os.Mkdir(session, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(session) })
	identity, err := runlayout.New(session, repositoryRoot, os.TempDir(), runID)
	if err != nil {
		t.Fatal(err)
	}
	_, _, runDirectory, evidenceDirectory, err := identity.OwnedPaths(false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(runDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := cleanupGateCPreparation(identity); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{runDirectory, evidenceDirectory} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("partial path remains: %s", path)
		}
	}
}
