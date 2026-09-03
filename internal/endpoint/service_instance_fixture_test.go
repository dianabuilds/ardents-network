package endpoint

import (
	"os"
	"path/filepath"
	"testing"
)

// serviceInstanceFixtureRoot creates the owner-only directory required by a
// Service Instance, independent of the test process umask.
func serviceInstanceFixtureRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "service-instance-root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

// alphaPersistentFloorRoot creates the owner-only directory required by
// alpha.OpenPersistentFloor, independent of the test process umask.
func alphaPersistentFloorRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "alpha-floor")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

// transitGrantRoot creates the owner-only directory required by transit grant
// issuer and admission fixtures, independent of the test process umask.
func transitGrantRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "transit-grant")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

// publicationStoreRoot creates the owner-only directory for a Publication root,
// independent of the test process umask.
func publicationStoreRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "publication")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

// reachabilityStoreRoot creates the owner-only directory for a reachability
// Store root, independent of the test process umask.
func reachabilityStoreRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "reachability-store")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

// credentialIssuerRoot creates the owner-only directory for a credential issuer root,
// independent of the test process umask.
func credentialIssuerRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "issuer-root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

// transitAcquisitionRoot creates the owner-only directory for a transit
// acquisition store root, independent of the test process umask.
func transitAcquisitionRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "transit-acquisition")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}
