//go:build linux

package resource

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func hostingFixture(t *testing.T) (string, *hostingReading, *time.Time) {
	t.Helper()
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	reading := &hostingReading{Boot: "boot-one", Interfaces: []hostingInterface{{Name: "eth0", Index: 2, Tx: 500, Rx: 800}}}
	root := filepath.Join(t.TempDir(), "hosting")
	policy := HostingPolicy{Provider: "declared fixture provider", Start: now.Add(-time.Hour), End: now.Add(time.Hour),
		Unit: "B", Quantity: 1000, Directions: "tx+rx", Interfaces: []string{"eth0"}, InitialUsedBytes: 100, LowWatermarkBytes: 100}
	if err := initializeHosting(root, policy, *reading, now); err != nil {
		t.Fatal(err)
	}
	return root, reading, &now
}

func openHostingFixture(t *testing.T, root string, reading *hostingReading, now *time.Time) *Hosting {
	t.Helper()
	owner, err := openHosting(root, func([]string) (hostingReading, error) { return *reading, nil }, func() time.Time { return *now })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	return owner
}

func TestHostingReservationsShareOneDurablePeriod(t *testing.T) {
	root, reading, now := hostingFixture(t)
	first := openHostingFixture(t, root, reading, now)
	second := openHostingFixture(t, root, reading, now)
	held, err := first.Reserve(t.Context(), HostingTraffic{Tx: 100, Rx: 200}, HostingTraffic{Tx: 10, Rx: 20}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.Reserve(t.Context(), HostingTraffic{Tx: 600}, HostingTraffic{Rx: 100}, now.Add(time.Minute)); err == nil {
		t.Fatal("another owner multiplied the host allowance")
	}
	other, err := second.Reserve(t.Context(), HostingTraffic{Tx: 220}, HostingTraffic{Rx: 30}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	reading.Interfaces[0].Tx += 40
	reading.Interfaces[0].Rx += 60
	if err := held.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := held.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	got, err := second.Observe(t.Context())
	if err != nil || got.UsedBytes != 200 || got.ReservedBytes != 250 || got.RemainingBytes != 550 {
		t.Fatalf("shared accounting or exact release differs: %+v / %v", got, err)
	}
	if err := other.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestHostingReopenDoesNotRefundAbandonedWork(t *testing.T) {
	root, reading, now := hostingFixture(t)
	first := openHostingFixture(t, root, reading, now)
	if _, err := first.Reserve(t.Context(), HostingTraffic{Tx: 200}, HostingTraffic{Rx: 100}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openHostingFixture(t, root, reading, now)
	got, err := reopened.Observe(t.Context())
	if err != nil || got.UsedBytes != 100 || got.ReservedBytes != 300 {
		t.Fatalf("restart refunded work: %+v / %v", got, err)
	}
	*now = now.Add(2 * time.Hour)
	got, err = reopened.Observe(t.Context())
	if err != nil || !got.Drain || got.ReservedBytes != 300 {
		t.Fatalf("period expiry reset accounting: %+v / %v", got, err)
	}
	if _, err := reopened.Reserve(t.Context(), HostingTraffic{Tx: 1}, HostingTraffic{Rx: 1}, now.Add(time.Second)); err == nil {
		t.Fatal("expired period admitted new work")
	}
}

func TestHostingStartsAnAlreadyExhaustedPeriodOnlyToDrain(t *testing.T) {
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	reading := hostingReading{Boot: "boot-one", Interfaces: []hostingInterface{{Name: "eth0", Index: 2, Tx: 500, Rx: 800}}}
	root := filepath.Join(t.TempDir(), "hosting")
	policy := HostingPolicy{Provider: "declared fixture provider", Start: now.Add(-time.Hour), End: now.Add(time.Hour),
		Unit: "B", Quantity: 1000, Directions: "tx+rx", Interfaces: []string{"eth0"}, InitialUsedBytes: 1000, LowWatermarkBytes: 100}
	if err := initializeHosting(root, policy, reading, now); err != nil {
		t.Fatalf("exhausted provider period must retain its drain floor: %v", err)
	}
	owner := openHostingFixture(t, root, &reading, &now)
	got, err := owner.Observe(t.Context())
	if err != nil || !got.Protect || !got.Drain || got.RemainingBytes != 0 {
		t.Fatalf("exhausted provider period must drain without a reset: %+v / %v", got, err)
	}
	if _, err := owner.Reserve(t.Context(), HostingTraffic{Tx: 1}, HostingTraffic{Rx: 1}, now.Add(time.Minute)); err == nil {
		t.Fatal("exhausted provider period admitted new work")
	}
}

func TestHostingLostCounterContinuityRefuses(t *testing.T) {
	for _, change := range []string{"boot", "interface", "tx", "clock"} {
		t.Run(change, func(t *testing.T) {
			root, reading, now := hostingFixture(t)
			owner := openHostingFixture(t, root, reading, now)
			switch change {
			case "boot":
				reading.Boot = "boot-two"
			case "interface":
				reading.Interfaces[0].Index++
			case "tx":
				reading.Interfaces[0].Tx--
			case "clock":
				*now = now.Add(-time.Second)
			}
			got, err := owner.Observe(t.Context())
			if err == nil || !got.Drain {
				t.Fatalf("unknown counter tail admitted: %+v / %v", got, err)
			}
		})
	}
}

func TestHostingCorruptStateIsNeverInitializedAgain(t *testing.T) {
	root, reading, now := hostingFixture(t)
	raw, err := os.ReadFile(filepath.Join(root, "period.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "period.json"), raw[:len(raw)/2], 0600); err != nil {
		t.Fatal(err)
	}
	if owner, err := openHosting(root, func([]string) (hostingReading, error) { return *reading, nil }, func() time.Time { return *now }); err == nil {
		_ = owner.Close()
		t.Fatal("corrupt period reopened")
	}
	if err := initializeHosting(root, HostingPolicy{}, *reading, *now); err == nil {
		t.Fatal("existing corrupt period was reset")
	}
}

func TestHostingCancellationPreventsReservation(t *testing.T) {
	root, reading, now := hostingFixture(t)
	owner := openHostingFixture(t, root, reading, now)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := owner.Reserve(canceled, HostingTraffic{Tx: 100}, HostingTraffic{Rx: 20}, now.Add(time.Minute)); err == nil {
		t.Fatal("canceled work admitted")
	}
	got, err := owner.Observe(t.Context())
	if err != nil || got.ReservedBytes != 0 {
		t.Fatalf("cancellation changed reserve: %+v / %v", got, err)
	}
}
