//go:build !linux && !darwin && !windows

package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnsupportedStoreRefusesBeforeFiles(t *testing.T) {
	parent := t.TempDir()
	store, err := Open(Config{Root: filepath.Join(parent, "absent"), Network: [32]byte{1}})
	if store != nil || err == nil {
		t.Fatal("unsupported Store accepted", store, err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatal("refusal changed native state", entries, err)
	}
}
