//go:build linux

package node

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

func closedForwardingHostingRoot(t *testing.T) string {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	root := filepath.Join(t.TempDir(), "hosting")
	policy := resource.HostingPolicy{Provider: "test fixture", Start: now.Add(-time.Hour), End: now.Add(time.Hour), Unit: "GiB", Quantity: 1,
		Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1 << 20}
	if err := resource.InitializeHosting(root, policy); err != nil {
		t.Fatal(err)
	}
	return root
}
