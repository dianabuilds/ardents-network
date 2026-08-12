package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBoundedDirectoryReturnsCanonicalOrder(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"0002.bin", "0001.bin", "0000.bin"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := readBoundedDirectory(root, 3)
	if err != nil {
		t.Fatal(err)
	}
	for index, want := range []string{"0000.bin", "0001.bin", "0002.bin"} {
		if entries[index].Name() != want {
			t.Fatalf("entry %d = %q, want %q", index, entries[index].Name(), want)
		}
	}
}
