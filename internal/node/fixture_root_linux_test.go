//go:build linux

package node

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	hostingbudget "github.com/dianabuilds/ardents-network/internal/hosting"
)

// closedIssuerFixtureRoot creates the owner-only directory for the finite
// closed issuer key root, independent of the test process umask.
func closedIssuerFixtureRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "closed-issuer")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func hostingFixtureRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "hosting")
	policy := hostingbudget.Policy{Provider: "test fixture", Start: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), Unit: "GiB", Quantity: 1,
		Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1 << 20}
	if err := hostingbudget.Initialize(root, policy); err != nil {
		t.Fatal(err)
	}
	return root
}
