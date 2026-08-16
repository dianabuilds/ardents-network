package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenRejectsIncompletePlanBeforeStateMutation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "entry.json")
	if err := os.WriteFile(path, []byte(`{"Schema":"ardents-h3-bridge-entry-plan-v1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if owner, err := loadEntryPlan(path); err == nil || owner != nil {
		t.Fatalf("Open() = %v, %v", owner, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("rejected plan state mutation = %v, %v", entries, err)
	}
}
