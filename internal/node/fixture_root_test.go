package node

import (
	"os"
	"path/filepath"
	"testing"
)

// localRoleStateRoot creates the owner-only directory for a local role state root,
// independent of the test process umask.
func localRoleStateRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "local-role-state")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

// transitIssuerStoreRoot creates the owner-only directory for a transit issuer root,
// independent of the test process umask.
func transitIssuerStoreRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "transit-issuer")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

// transitGrantRoot creates the owner-only directory for a transit grant root,
// independent of the test process umask.
func transitGrantRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "transit-grant")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

// reachabilityStoreRoot creates the owner-only directory for a reachability
// store root, independent of the test process umask.
func reachabilityStoreRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "reachability-store")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

// entryAdmitterRoot creates the owner-only directory for an entry admitter root,
// independent of the test process umask.
func entryAdmitterRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "entry-admitter")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}
