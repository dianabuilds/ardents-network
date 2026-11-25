package processfixture

import (
	"path/filepath"
	"testing"
)

// TempDir returns the actual path of a test-owned directory. Persistent owners
// reject symlinked ancestors, including operating-system temporary-dir aliases.
func TempDir(t testing.TB) string {
	t.Helper()
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal("resolve test directory:", err)
	}
	return path
}
