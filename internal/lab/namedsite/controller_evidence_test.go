package namedsite

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicBoundedJSONPublishesOnlyCompleteEvidence(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "publication.json")
	want := map[string]string{"status": "published"}
	if err := writeAtomicBoundedJSON(path, want); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !json.Valid(data) {
		t.Fatalf("atomic evidence = %q, %v", data, err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "publication.json" {
		t.Fatalf("atomic evidence left temporary entries: %v", entries)
	}
}

func TestAtomicBoundedJSONCleansTemporaryFileAfterRenameFailure(t *testing.T) {
	directory := t.TempDir()
	destination := filepath.Join(directory, "publication.json")
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomicBoundedJSON(destination, map[string]string{"status": "published"}); err == nil {
		t.Fatal("rename onto a directory unexpectedly succeeded")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "publication.json" || !entries[0].IsDir() {
		t.Fatalf("rename failure left temporary evidence: %v", entries)
	}
}
