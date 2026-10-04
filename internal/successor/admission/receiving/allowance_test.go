package receiving_test

import (
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"testing"
	"time"
)

func TestAllowancePreservesOriginalDeadlineAndReplacesRemaining(t *testing.T) {
	now := time.Unix(1800000100, 0)
	a, err := receiving.NewAllowance(admission.ForwardClass, now, now.Add(time.Minute), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	b, err := a.Replenish(now.Add(time.Second), 1)
	if err != nil || b.Bytes() != 32<<20 || b.Deadline() != now.Add(time.Minute) {
		t.Fatalf("refill=%+v err=%v", b, err)
	}
	if _, err = a.Replenish(now, 0); err == nil {
		t.Fatal("resurrected exhausted allowance")
	}
	if _, err = a.Replenish(a.Deadline(), 1); err == nil {
		t.Fatal("extended expired allowance")
	}
}

func TestRegistrationAllowanceKeepsSelectedMaximumAndEarlierBounds(t *testing.T) {
	now := time.Unix(1800000100, 0)
	for _, test := range []struct {
		name               string
		caller, duty, want time.Duration
	}{
		{"selected lifetime", time.Hour, time.Hour, 600 * time.Second},
		{"caller", 15 * time.Second, time.Hour, 15 * time.Second},
		{"authority", time.Hour, 20 * time.Second, 20 * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			a, err := receiving.NewAllowance(admission.RegistrationClass, now, now.Add(test.caller), now.Add(test.duty))
			if err != nil || a.Bytes() != 1048576 || !a.Deadline().Equal(now.Add(test.want)) {
				t.Fatalf("selected Registration contract: bytes=%d deadline=%s error=%v", a.Bytes(), a.Deadline(), err)
			}
			if _, err := a.Replenish(now, 1); err == nil {
				t.Fatal("Registration acquired forwarding refill authority")
			}
		})
	}
}

func TestVerificationGateBoundsConcurrencyRateAndClockRollback(t *testing.T) {
	var gate receiving.VerificationGate
	now := time.Unix(1800000100, 0)
	var releases []func()
	for range 4 {
		release, err := gate.Begin(now)
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	if _, err := gate.Begin(now); err == nil {
		t.Fatal("fifth concurrent verification")
	}
	for _, release := range releases {
		release()
		release()
	}
	for range 124 {
		release, err := gate.Begin(now)
		if err != nil {
			t.Fatal(err)
		}
		release()
	}
	if _, err := gate.Begin(now); err == nil {
		t.Fatal("129th verification")
	}
	if _, err := gate.Begin(now.Add(-time.Second)); err == nil {
		t.Fatal("clock rollback reset quota")
	}
	release, err := gate.Begin(now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	release()
}
