//go:build !linux && !darwin

package localsnapshot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadRefusesExistingFileOnUnsupportedPlatform(t *testing.T) {
	path := filepath.Join(t.TempDir(), "document")
	if err := os.WriteFile(path, []byte("selected"), 0600); err != nil {
		t.Fatal(err)
	}
	body, err := Read(t.Context(), path)
	if err == nil || body != nil {
		t.Fatalf("unsupported import returned %d bytes, error %v", len(body), err)
	}
}
