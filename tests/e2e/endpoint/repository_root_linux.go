//go:build linux && (endpoint_pair_native || endpoint_launch_native)

package endpoint

import (
	"os"
	"path/filepath"
	"testing"
)

// RepositoryRoot locates the module from an Endpoint qualification driver.
func RepositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Ardents module root not found")
		}
		dir = parent
	}
}
