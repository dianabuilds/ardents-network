package duty

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/durablefile"
)

func ownedRootFixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "owned-root")
	if err := durablefile.InspectRoot(root, true, "missing", "invalid"); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeOwnedFixture(path string, raw []byte) error {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return durablefile.WriteExclusive(path, raw)
	}
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}
