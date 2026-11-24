//go:build linux && node_process_native

package processfixture

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestPinnedCommandRejectsReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "command")
	original := []byte("original command fixture")
	want := fmt.Sprintf("%x", sha256.Sum256(original))
	if err := os.WriteFile(path, original, 0700); err != nil {
		t.Fatal(err)
	}
	if err := verifyPinnedCommand(path, want); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("changed fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := verifyPinnedCommand(path, want); err == nil {
		t.Fatal("accepted replaced command bytes")
	}
}
