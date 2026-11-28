package duty

import (
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
