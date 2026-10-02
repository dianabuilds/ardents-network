//go:build linux

package hosting

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCopiedReservationCannotRefundAnotherHandle(t *testing.T) {
	root, reading, now := hostingFixture(t)
	b := openHostingFixture(t, root, reading, now)
	first, err := b.Reserve(t.Context(), Traffic{Tx: 100}, Traffic{Rx: 20}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	other, err := b.Reserve(t.Context(), Traffic{Tx: 50}, Traffic{Rx: 10}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	copyOfFirst := *first
	if err = first.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err = copyOfFirst.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	view, err := b.Observe(t.Context())
	if err != nil || view.ReservedBytes != 60 {
		t.Fatalf("copy refunded other: %+v %v", view, err)
	}
	if err = other.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestReserveBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name           string
		work, terminal Traffic
		delta          time.Duration
		ok             bool
	}{
		{"exact-watermark", Traffic{Tx: 700}, Traffic{Rx: 100}, time.Minute, true},
		{"watermark-crossed", Traffic{Tx: 701}, Traffic{Rx: 100}, time.Minute, false},
		{"zero-work", Traffic{}, Traffic{Tx: 1}, time.Minute, false},
		{"zero-terminal", Traffic{Tx: 1}, Traffic{}, time.Minute, false},
		{"expired", Traffic{Tx: 1}, Traffic{Rx: 1}, 0, false},
		{"period-end", Traffic{Tx: 1}, Traffic{Rx: 1}, time.Hour, true},
		{"past-period", Traffic{Tx: 1}, Traffic{Rx: 1}, time.Hour + time.Nanosecond, false},
		{"sum-overflow", Traffic{Tx: math.MaxUint64}, Traffic{Rx: 1}, time.Minute, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, reading, now := hostingFixture(t)
			b := openHostingFixture(t, root, reading, now)
			held, err := b.Reserve(t.Context(), tc.work, tc.terminal, now.Add(tc.delta))
			if (err == nil) != tc.ok {
				t.Fatalf("reserve: %v", err)
			}
			if held != nil {
				if err = held.Release(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
			view, err := b.Observe(t.Context())
			if err != nil || view.ReservedBytes != 0 {
				t.Fatalf("refusal changed state: %+v %v", view, err)
			}
		})
	}
}

func TestReserveUsesProviderDirections(t *testing.T) {
	for _, tc := range []struct {
		direction string
		cost      uint64
	}{{"tx", 110}, {"rx", 220}, {"tx+rx", 330}} {
		t.Run(tc.direction, func(t *testing.T) {
			now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
			reading := hostingReading{Boot: "fixture-boot", Interfaces: []hostingInterface{{Name: "lo", Index: 1}}}
			policy := Policy{Provider: "fixture", Start: now.Add(-time.Hour), End: now.Add(time.Hour), Unit: "B", Quantity: 1000, Directions: tc.direction, Interfaces: []string{"lo"}, LowWatermarkBytes: 100}
			root := filepath.Join(t.TempDir(), "budget")
			if err := initializeHosting(root, policy, reading, now); err != nil {
				t.Fatal(err)
			}
			b, err := openHosting(root, func([]string) (hostingReading, error) { return reading, nil }, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			held, err := b.Reserve(t.Context(), Traffic{Tx: 100, Rx: 200}, Traffic{Tx: 10, Rx: 20}, now.Add(time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			view, err := b.Observe(t.Context())
			if err != nil || view.ReservedBytes != tc.cost {
				t.Fatalf("direction lost: %+v %v", view, err)
			}
			if err = held.Release(t.Context()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestForeignAndPendingStateRefusedWithoutMutation(t *testing.T) {
	for _, file := range []string{"budget.pending", "budget.pin", "budget.lock", "budget.json"} {
		t.Run(file, func(t *testing.T) {
			root, reading, now := hostingFixture(t)
			path := filepath.Join(root, file)
			if file == "budget.pending" {
				if err := os.WriteFile(path, []byte("interrupted"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("outside", path); err != nil {
					t.Fatal(err)
				}
			}
			if b, err := openHosting(root, func([]string) (hostingReading, error) { return *reading, nil }, func() time.Time { return *now }); err == nil {
				b.Close()
				t.Fatal("ambiguous state opened")
			}
		})
	}
	root, reading, now := hostingFixture(t)
	for _, file := range []string{"pin", "lock", "json"} {
		if err := os.Rename(filepath.Join(root, "budget."+file), filepath.Join(root, "period."+file)); err != nil {
			t.Fatal(err)
		}
	}
	if b, err := openHosting(root, func([]string) (hostingReading, error) { return *reading, nil }, func() time.Time { return *now }); err == nil {
		b.Close()
		t.Fatal("legacy root opened")
	}
	if _, err := os.Stat(filepath.Join(root, "budget.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("foreign root changed")
	}
}
