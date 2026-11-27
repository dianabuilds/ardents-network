package socketfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRetireRemovesOnlyOriginalSocket(t *testing.T) {
	parent, err := os.MkdirTemp("", "af-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(parent); err != nil {
			t.Error(err)
		}
	})
	path := filepath.Join(parent, "own.sock")
	listener, entry, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	defer entry.parent.Close()
	if other, _, err := Listen(path); other != nil || !errors.Is(err, ErrOccupied) {
		t.Fatal("occupied socket path was reused", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); err != nil {
		t.Fatal("listener close removed the entry before retirement", err)
	}
	if err := entry.Retire(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("original socket entry remains after retirement", err)
	}
}
