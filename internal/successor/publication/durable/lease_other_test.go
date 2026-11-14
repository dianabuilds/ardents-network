//go:build !linux && !windows

package durable

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnsupportedPublicationRootRefusesBeforeFiles(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "absent")
	root, err := Open(t.Context(), Config{Root: path, Target: [32]byte{1}, Network: [32]byte{2}})
	if root != nil || err == nil {
		t.Fatal("unsupported native root accepted", root, err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatal("refusal created native state", entries, err)
	}
}
