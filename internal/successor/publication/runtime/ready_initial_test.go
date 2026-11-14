package runtime

import "testing"

// Compaction must rebuild the ready queue from the retained map, keeping only
// wireJoined unclaimed deliveries and dropping stale/claimed/pending entries
// (audit DD-6).
func TestCompactReadyInitialRebuildsFromRetainedMap(t *testing.T) {
	p := &Publisher{initialDeliveries: make(map[[32]byte]*initialDelivery)}
	kept := &initialDelivery{nonce: [32]byte{1}, wireJoined: true}
	p.initialDeliveries[kept.nonce] = kept
	claimed := &initialDelivery{nonce: [32]byte{2}, wireJoined: true, serviceClaimed: true}
	p.initialDeliveries[claimed.nonce] = claimed
	pending := &initialDelivery{nonce: [32]byte{3}}
	p.initialDeliveries[pending.nonce] = pending
	stale := &initialDelivery{nonce: [32]byte{4}, wireJoined: true} // not retained
	p.readyInitial = []*initialDelivery{stale, kept, claimed, pending}
	p.compactReadyInitial()
	if len(p.readyInitial) != 1 || p.readyInitial[0] != kept {
		t.Fatal("compaction did not rebuild the eligible retained candidates")
	}
}
