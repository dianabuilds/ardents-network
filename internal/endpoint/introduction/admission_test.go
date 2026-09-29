//go:build linux

package introduction

import (
	"testing"
	"time"
)

func TestAdmissionOpeningRateAndReplayRetention(t *testing.T) {
	var admission Admission
	now := time.Unix(1_700_000_000, 0).UTC()
	nonce := func(value byte) [32]byte { return [32]byte{value} }

	for value := byte(1); value <= 4; value++ {
		if err := admission.ReserveOpeningLocked(nonce(value), now); err != nil {
			t.Fatal(err)
		}
	}
	if err := admission.ReserveOpeningLocked(nonce(8), now.Add(time.Second-time.Nanosecond)); err == nil {
		t.Fatal("more than four openings in one second")
	}
	if err := admission.ReserveOpeningLocked(nonce(8), now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := admission.ReserveOpeningLocked(nonce(9), now); err == nil {
		t.Fatal("clock rollback reopened rate allowance")
	}

	admission.RetainAcceptedLocked(nonce(10), now.Add(2*time.Second))
	if err := admission.ReserveOpeningLocked(nonce(11), now.Add(61*time.Second)); err != nil {
		t.Fatalf("opening rate did not recover before replay expiry: %v", err)
	}
	if err := admission.ReserveOpeningLocked(nonce(10), now.Add(61*time.Second)); err == nil {
		t.Fatal("accepted nonce replayed before retention expiry")
	}
	if err := admission.ReserveOpeningLocked(nonce(10), now.Add(62*time.Second)); err != nil {
		t.Fatalf("expired replay nonce remained unavailable: %v", err)
	}
}
