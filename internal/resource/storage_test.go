package resource

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagedStorageMeasurementIsBoundedByBytesAndFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "state"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "first"), []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "state", "second"), []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, files, err := measureManagedStorage([]string{root}, 10, 4); err != nil || got != 8 || files != 2 {
		t.Fatalf("storage = %d bytes/%d files, %v; want 8 bytes/2 files", got, files, err)
	}
	if _, _, err := measureManagedStorage([]string{root}, 7, 4); err == nil {
		t.Fatal("byte ceiling was not enforced")
	}
	if _, _, err := measureManagedStorage([]string{root}, 10, 1); err == nil {
		t.Fatal("file ceiling was not enforced")
	}
}
