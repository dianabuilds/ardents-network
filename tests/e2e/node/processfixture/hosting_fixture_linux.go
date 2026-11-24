package processfixture

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/hosting"
)

// ProcessBudget initializes a fresh loopback Hosting budget for an original process.
func ProcessBudget(t testing.TB) string {
	t.Helper()
	return ProcessBudgetWithCapacity(t, 100)
}

// ProcessBudgetWithCapacity provisions multiple simultaneous role reservations when required.
func ProcessBudgetWithCapacity(t testing.TB, capacity uint64) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "budget")
	start := time.Now().UTC().Truncate(time.Hour)
	if err := hosting.Initialize(root, hosting.Policy{Provider: "Route process test", Start: start, End: start.Add(2 * time.Hour), Unit: "MiB", Quantity: capacity, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1000}); err != nil {
		t.Fatal(err)
	}
	return root
}
