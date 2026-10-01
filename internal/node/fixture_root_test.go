package node

import (
	"os"
	"path/filepath"
	"testing"
)

// localRoleStateRoot creates the owner-only directory for a local role state root,
// independent of the test process umask.
func localRoleStateRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "local-role-state")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}
