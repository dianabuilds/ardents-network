package siteexperiment

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReferenceSocketsReadyRejectsWrongFilesystemType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.sock")
	if err := os.WriteFile(path, []byte("not a socket"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := referenceSocketsReady(path); err == nil {
		t.Fatal("regular file was accepted as a reference Site socket")
	}
}

func TestReferenceSocketsReadyWaitsOnlyForAbsentPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.sock")
	ready, err := referenceSocketsReady(path)
	if err != nil || ready {
		t.Fatalf("referenceSocketsReady() = %t, %v", ready, err)
	}
}
