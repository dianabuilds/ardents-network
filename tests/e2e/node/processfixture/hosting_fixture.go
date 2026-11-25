package processfixture

import (
	"net"
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
	root := filepath.Join(TempDir(t), "budget")
	start := time.Now().UTC().Truncate(time.Hour)
	// Route fixtures use 127.0.0.1; the interface name is a host fact.
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatal("loopback interface inventory unavailable:", err)
	}
	loopback := ""
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback == 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			t.Fatal("loopback interface addresses unavailable:", err)
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && ip.Equal(net.IPv4(127, 0, 0, 1)) {
				loopback = iface.Name
				break
			}
		}
		if loopback != "" {
			break
		}
	}
	if loopback == "" {
		t.Fatal("invalid environment: 127.0.0.1 loopback interface unavailable")
	}
	if err := hosting.Initialize(root, hosting.Policy{Provider: "Route process test", Start: start, End: start.Add(2 * time.Hour), Unit: "MiB", Quantity: capacity, Directions: "tx+rx", Interfaces: []string{loopback}, LowWatermarkBytes: 1000}); err != nil {
		t.Fatal(err)
	}
	return root
}
