//go:build !linux && !darwin

package targetlink

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateFileRefusesUnsupportedHost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "target-link")
	if err := os.WriteFile(path, []byte("explicit-link\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPrivate(path)
	if got != "" || err == nil || err.Error() != "private Target Link file is unavailable" {
		t.Fatalf("unsupported private file accepted: %q, %v", got, err)
	}
}
