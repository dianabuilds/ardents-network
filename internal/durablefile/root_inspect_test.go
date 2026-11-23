package durablefile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectRootCreatesMissingWhenAllowed(t *testing.T) {
	root := filepath.Join(t.TempDir(), "new", "root")
	if err := InspectRoot(root, true, "missing", "not a directory"); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(root); err != nil || !info.IsDir() {
		t.Fatalf("root not created: %v", err)
	}
}

func TestInspectRootRefusesMissingWhenNotAllowed(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent")
	err := InspectRoot(root, false, "missing refusal", "not a directory")
	if err == nil || err.Error() != "missing refusal" {
		t.Fatalf("missing root = %v; want caller missing refusal", err)
	}
	if _, statErr := os.Lstat(root); !os.IsNotExist(statErr) {
		t.Fatal("refused missing root was created")
	}
}

func TestInspectRootRefusesNonDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(root, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	err := InspectRoot(root, true, "missing", "not a directory refusal")
	if err == nil || err.Error() != "not a directory refusal" {
		t.Fatalf("non-directory root = %v; want caller not-a-directory refusal", err)
	}
}
