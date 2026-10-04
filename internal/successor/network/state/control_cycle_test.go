package state

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"reflect"
	"testing"
)

// The persisted ARDS1D4 record is a compatibility boundary even when its
// in-memory status names change. This fixture exercises every cycle section.
func TestDistributionJournalCanonicalCycleBytes(t *testing.T) {
	state := distributionState{
		sequence: 7, epochFloor: 3, epochDigest: [32]byte{1, 2, 3},
		trustedTimeFloor: 1_800_000_010, conflicting: true,
		consecutiveFailures: 2, backoffLevel: 1, nextAutomatic: 1_800_000_070,
		history: [][32]byte{{4}, {5}},
		cycleID: 9, cycleActive: true, cyclePurpose: sourceCyclePurposeRefresh,
		cycleStarted: 1_800_000_010, cycleDeadline: 1_800_000_025,
		attempts: [4]byte{sourceAttemptCompleted, sourceAttemptFailed, sourceAttemptInFlight, sourceAttemptNotStarted}, outcomes: [4]byte{sourceOutcomeValid, sourceOutcomeTimeout, 0, 0},
		requestedDigests: [2][32]byte{{6}},
		observedEpochs:   [4]uint64{3, 2}, observedDigests: [4][32]byte{{7}, {8}},
		pendingDigest: [32]byte{9}, pendingValidFrom: 1_800_000_030,
		cycleSeed: [32]byte{10}, sourceOrder: [2]byte{1, 0},
	}
	raw := encodeDistributionState(state)
	if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != "e74d50d88e3258000a82f0f0b9eb132103f47a7d4c07392bc85806c186ae27c6" || len(raw) != 479 {
		t.Fatalf("ARDS1D4 encoding changed: sha256=%s bytes=%d", got, len(raw))
	}
	restored, err := decodeDistributionState(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored, state) || !bytes.Equal(encodeDistributionState(restored), raw) {
		t.Fatal("distribution journal round trip changed state or bytes")
	}
	for _, test := range []struct {
		name   string
		change func(*distributionState)
	}{
		{"unknown purpose", func(state *distributionState) { state.cyclePurpose = 2 }},
		{"unset active purpose", func(state *distributionState) { state.cyclePurpose = 0 }},
		{"unknown attempt", func(state *distributionState) { state.attempts[0] = 4 }},
		{"missing digest selector", func(state *distributionState) { state.requestedDigests[0] = [32]byte{} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			invalid := state
			test.change(&invalid)
			if _, err := decodeDistributionState(encodeDistributionState(invalid)); err == nil {
				t.Fatal("accepted invalid persisted cycle")
			}
		})
	}
}
