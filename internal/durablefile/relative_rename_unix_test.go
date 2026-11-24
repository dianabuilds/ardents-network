//go:build linux || darwin

package durablefile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenameatRenamesRetainedChildWithinHeldDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "stage"), []byte("payload"), 0600); err != nil {
		t.Fatal(err)
	}
	directory, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	if err := Renameat(int(directory.Fd()), "stage", int(directory.Fd()), "final"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "final"))
	if err != nil || string(body) != "payload" {
		t.Fatalf("retained rename result: %q %v", body, err)
	}
	if _, err := os.Lstat(filepath.Join(root, "stage")); !os.IsNotExist(err) {
		t.Fatalf("staged name survived rename: %v", err)
	}
}

// The rename must act on the held directory inode, never a pathname that a
// concurrent move could have replaced: after the original root is moved away
// and a foreign root is created at the same path, the retained rename still
// lands inside the held (moved) directory.
func TestRenameatStaysAnchoredAfterRootMoveAndReplacement(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "stage"), []byte("held"), 0600); err != nil {
		t.Fatal(err)
	}
	directory, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	moved := root + ".moved"
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := Renameat(int(directory.Fd()), "stage", int(directory.Fd()), "final"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(moved, "final"))
	if err != nil || string(body) != "held" {
		t.Fatalf("anchored rename lost held inode: %q %v", body, err)
	}
	if _, err := os.Lstat(filepath.Join(root, "final")); !os.IsNotExist(err) {
		t.Fatalf("rename leaked into replacement root: %v", err)
	}
}

func TestRenameatRefusesAbsentChild(t *testing.T) {
	root := t.TempDir()
	directory, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	if err := Renameat(int(directory.Fd()), "absent", int(directory.Fd()), "final"); err == nil {
		t.Fatal("absent child renamed")
	}
}
