//go:build linux

package hosting

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSharedSnapshotFreshnessAndReservationVisibility(t *testing.T) {
	root, reading, now := hostingFixture(t)
	first := openHostingFixture(t, root, reading, now)
	second := openHostingFixture(t, root, reading, now)
	first.measure = func([]string) (hostingReading, error) {
		t.Fatal("fresh sample measured again")
		return hostingReading{}, nil
	}
	sample, err := first.Sample(t.Context(), time.Second)
	if err != nil || sample.ObservedAt() != *now {
		t.Fatal(sample, err)
	}
	held, err := second.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 20}, Termination: Traffic{Rx: 10}, WorkUntil: now.Add(time.Minute), HoldUntil: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	sample, err = first.Sample(t.Context(), time.Second)
	if err != nil || sample.Observation().ReservedBytes != 30 {
		t.Fatal("sample hid reservation", sample, err)
	}
	changed := sample.Observation()
	changed.ReservedBytes = 0
	if sample.Observation().ReservedBytes != 30 {
		t.Fatal("sample aliases caller")
	}
	*now = now.Add(2 * time.Second)
	first.measure = func([]string) (hostingReading, error) { return *reading, nil }
	sample, err = first.Sample(t.Context(), time.Second)
	if err != nil || sample.ObservedAt() != *now {
		t.Fatal("stale sample reused", sample, err)
	}
	if err := held.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Sample(t.Context(), time.Second); err == nil {
		t.Fatal("closed handle reused observation")
	}
	if _, err := second.Sample(t.Context(), time.Second); err != nil {
		t.Fatal("independent handle closed", err)
	}
	if err := os.WriteFile(filepath.Join(root, "budget.pending"), []byte("uncertain"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Sample(t.Context(), time.Second); err == nil {
		t.Fatal("fresh sample hid uncertain storage")
	}
}

func TestSampleDoesNotRenewExpiredPeriod(t *testing.T) {
	root, reading, now := hostingFixture(t)
	owner := openHostingFixture(t, root, reading, now)
	*now = now.Add(time.Hour)
	sample, err := owner.Sample(t.Context(), time.Second)
	if err != nil || !sample.Observation().Drain || sample.ValidUntil().After(*now) {
		t.Fatal(sample, err)
	}
}
