package inspection

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareInspectionRootClaimsOnlyEmptyOwnedRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "inspection")
	catalog, release, network, err := prepareInspectionRoot(root)
	if err != nil || catalog != filepath.Join(root, "catalog") || release != filepath.Join(root, "release") || network != filepath.Join(root, "network") {
		t.Fatalf("prepare inspection root = %q, %q, %q, %v", catalog, release, network, err)
	}
	if err := os.Mkdir(catalog, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := prepareInspectionRoot(root); err != nil {
		t.Fatalf("reopen owned inspection root: %v", err)
	}
	foreign := filepath.Join(t.TempDir(), "foreign")
	if err := os.Mkdir(foreign, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "endpoint-state"), []byte("foreign"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := prepareInspectionRoot(foreign); err == nil {
		t.Fatal("foreign inspection root was claimed")
	}
}
