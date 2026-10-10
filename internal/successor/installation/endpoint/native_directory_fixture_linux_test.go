//go:build installation_native

package endpoint

import (
	"os"
	"path/filepath"
	"testing"
)

// This selected profile exercises actual root ownership and filesystem changes.
// It neither invokes systemd nor grants installed generation authority.
func nativeRequestDirectory(t *testing.T) string {
	t.Helper()
	parent := os.Getenv("ARDENTS_INSTALLATION_NATIVE_ROOT")
	if os.Geteuid() != 0 || parent == "" {
		t.Fatal("invalid environment: installation_native requires root and an explicit trusted temporary parent")
	}
	if _, err := rootDirectoryAncestors(parent); err != nil {
		t.Fatalf("invalid environment: untrusted temporary parent: %v", err)
	}
	directory, err := os.MkdirTemp(parent, "installation-request-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	return directory
}

func writeNativeRequest(t *testing.T, directory string) string {
	t.Helper()
	filename := filepath.Join(directory, "request.json")
	if err := os.WriteFile(filename, requestFixture(), 0600); err != nil {
		t.Fatal(err)
	}
	return filename
}
