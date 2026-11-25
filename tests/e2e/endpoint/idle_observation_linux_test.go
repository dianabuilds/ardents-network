//go:build linux && endpoint_launch_native

package endpoint

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/hosting"
)

func TestNET32IdleFreshBoundaryBypassesRecentHostingSample(t *testing.T) {
	root := filepath.Join(t.TempDir(), "hosting")
	now := time.Now().UTC().Truncate(time.Second)
	if err := hosting.Initialize(root, hosting.Policy{
		Provider: "NET-32 fresh boundary fixture", Start: now.Add(-time.Hour), End: now.Add(time.Hour),
		Unit: "MiB", Quantity: 64, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1,
	}); err != nil {
		t.Fatal(err)
	}
	budget, err := hosting.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := budget.Close(); err != nil {
			t.Error(err)
		}
	})
	initial, err := budget.Sample(t.Context(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := readNet32Sample(t.Context(), budget, root, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !fresh.hostingAt.After(initial.ObservedAt()) {
		t.Fatalf("boundary reused the recent persisted sample: initial %v fresh %v", initial.ObservedAt(), fresh.hostingAt)
	}
}
