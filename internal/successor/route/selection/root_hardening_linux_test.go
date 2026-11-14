//go:build linux

package selection

import (
	"os"
	"testing"
)

// The owner check must refuse a root owned by another user even when its
// permission bits are owner-only (audit SW-9). Requires root to chown.
func TestValidateOwnerOnlyRootRefusesForeignOwner(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("foreign-uid refusal requires root to chown the root")
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(root, 65534, 65534); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateOwnerOnlyRoot(root, info); err == nil {
		t.Fatal("foreign-uid root accepted")
	}
}

func TestValidateOwnerOnlyRootAcceptsEndpointOwner(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateOwnerOnlyRoot(root, info); err != nil {
		t.Fatal("endpoint-owned root refused", err)
	}
}
