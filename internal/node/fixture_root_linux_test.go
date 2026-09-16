//go:build linux

package node

import (
	"os"
	"path/filepath"
	"testing"
)

// closedIssuerFixtureRoot creates the owner-only directory for the finite
// closed issuer key root, independent of the test process umask.
func closedIssuerFixtureRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "closed-issuer")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}
