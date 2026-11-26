package durablefile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteExclusiveCreatesThenRefusesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "marker")
	if err := WriteExclusive(path, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != "first" {
		t.Fatalf("contents = %q, %v", raw, err)
	}
	if err := WriteExclusive(path, []byte("second")); err == nil {
		t.Fatal("exclusive write replaced an existing target")
	}
}

func TestReplaceFileReplacesDurablyAndCleansStaging(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "current"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceFile(root, ".current-", "current", []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(filepath.Join(root, "current")); err != nil || string(raw) != "new\n" {
		t.Fatalf("replaced contents = %q, %v", raw, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "current" {
			t.Fatalf("staging residue left behind: %s", entry.Name())
		}
	}
}

func TestCleanupGenerationsKeepsCurrentAndPrevious(t *testing.T) {
	root := t.TempDir()
	current := SHA256Hex([]byte("current"))
	previous := SHA256Hex([]byte("previous"))
	foreign := SHA256Hex([]byte("foreign"))
	for _, name := range []string{current, previous, foreign} {
		if err := os.WriteFile(filepath.Join(root, "state-"+name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "watermark"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "state-not-a-digest"), []byte("prune"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "state-directory"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := CleanupGenerations(root, current, previous); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "state-"+foreign)); !os.IsNotExist(err) {
		t.Fatalf("foreign generation retained: %v", err)
	}
	for _, name := range []string{current, previous} {
		if _, err := os.Stat(filepath.Join(root, "state-"+name)); err != nil {
			t.Fatalf("retained generation removed: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "watermark")); err != nil {
		t.Fatalf("non-generation entry removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "state-not-a-digest")); !os.IsNotExist(err) {
		t.Fatalf("unretained state entry remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "state-directory")); err != nil {
		t.Fatalf("directory removed: %v", err)
	}
}
