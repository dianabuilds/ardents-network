//go:build linux || darwin

package socketfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Unix hosts permit renaming the directory while the original socket and
// directory descriptor are open. Both entries remain physically observable.
func TestRetireUsesOriginalParentAfterRename(t *testing.T) {
	base, err := os.MkdirTemp("", "ar-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(base); err != nil {
			t.Error(err)
		}
	})
	parent := filepath.Join(base, "live")
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "one.sock")
	listener, entry, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	defer entry.parent.Close()

	moved := filepath.Join(base, "old")
	if err := os.Rename(parent, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	const replacement = "independent file in a replacement parent"
	if err := os.WriteFile(path, []byte(replacement), 0600); err != nil {
		t.Fatal(err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := entry.Retire(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(moved, "one.sock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("retirement lost the original parent", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != replacement {
		t.Fatal("retirement followed the replacement parent", err)
	}
}
