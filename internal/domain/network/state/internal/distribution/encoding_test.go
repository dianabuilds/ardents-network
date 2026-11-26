package distribution

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
	state := Record{
		Sequence: 7, EpochFloor: 3, EpochDigest: [32]byte{1, 2, 3},
		TrustedTimeFloor: 1_800_000_010, Conflicting: true,
		ConsecutiveFailures: 2, BackoffLevel: 1, NextAutomatic: 1_800_000_070,
		History: [][32]byte{{4}, {5}},
		CycleID: 9, CycleActive: true, CyclePurpose: Refresh,
		CycleStarted: 1_800_000_010, CycleDeadline: 1_800_000_025,
		Attempts: [4]byte{AttemptCompleted, AttemptFailed, AttemptInFlight, AttemptNotStarted}, Outcomes: [4]byte{OutcomeValid, OutcomeTimeout, 0, 0},
		RequestedDigests: [2][32]byte{{6}},
		ObservedEpochs:   [4]uint64{3, 2}, ObservedDigests: [4][32]byte{{7}, {8}},
		PendingDigest: [32]byte{9}, PendingValidFrom: 1_800_000_030,
		CycleSeed: [32]byte{10}, SourceOrder: [2]byte{1, 0},
	}
	raw := Encode(state)
	if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != "e74d50d88e3258000a82f0f0b9eb132103f47a7d4c07392bc85806c186ae27c6" || len(raw) != 479 {
		t.Fatalf("ARDS1D4 encoding changed: sha256=%s bytes=%d", got, len(raw))
	}
	restored, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored, state) || !bytes.Equal(Encode(restored), raw) {
		t.Fatal("distribution journal round trip changed state or bytes")
	}
	for _, test := range []struct {
		name   string
		change func(*Record)
	}{
		{"unknown purpose", func(state *Record) { state.CyclePurpose = 2 }},
		{"unset active purpose", func(state *Record) { state.CyclePurpose = 0 }},
		{"unknown attempt", func(state *Record) { state.Attempts[0] = 4 }},
		{"missing digest selector", func(state *Record) { state.RequestedDigests[0] = [32]byte{} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			invalid := state
			test.change(&invalid)
			if _, err := Decode(Encode(invalid)); err == nil {
				t.Fatal("accepted invalid persisted cycle")
			}
		})
	}
}
