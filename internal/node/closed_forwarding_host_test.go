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
	root := filepath.Join(t.TempDir(), "hosting")
	policy := resource.HostingPolicy{Provider: "test fixture", Start: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), Unit: "GiB", Quantity: 1,
		Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1 << 20}
	if err := resource.InitializeHosting(root, policy); err != nil {
		t.Fatal(err)
	}
	return root
}
