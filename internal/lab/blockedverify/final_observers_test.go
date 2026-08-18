package blockedverify

import "testing"

func TestFinalRawObserverEvidenceRequiresEveryBoundaryAndBatch(t *testing.T) {
	sets := []finalRawObserverSet{validFinalRawObserverSet()}
	if !validFinalRawObserverEvidence("profile/C1/00", sets) {
		t.Fatal("complete raw observer set was rejected")
	}
	sets[0].Observers = sets[0].Observers[:len(sets[0].Observers)-1]
	if validFinalRawObserverEvidence("profile/C1/00", sets) {
		t.Fatal("missing raw observer boundary was accepted")
	}
	if validFinalRawObserverEvidence("pressure/P4", []finalRawObserverSet{validFinalRawObserverSet()}) {
		t.Fatal("P4 accepted fewer than ten retained observer sets")
	}
}

func validFinalRawObserverSet() finalRawObserverSet {
	controls := map[string]finalRawDNSControl{"control": {IPv4UDP: 2, IPv6UDP: 2, IPv4TCP: 2,
		IfIndex: 1, Token: "0123456789abcdef0123456789abcdef"}}
	set := finalRawObserverSet{Observers: make([]finalRawObserver, 0, len(finalObserverBoundaries))}
	for _, boundary := range finalObserverBoundaries {
		role := boundary
		if boundary == "endpoint-adapter" {
			role = "endpoint"
		}
		set.Observers = append(set.Observers, finalRawObserver{Boundary: boundary, Role: role,
			Path: finalRawPathObservation{Phase: "s5.3-" + boundary, Passed: true},
			DNS: finalRawDNSObservation{Controls: 6, IPv4UDPControls: 2, IPv6UDPControls: 2,
				IPv4TCPControls: 2, BoundaryControls: controls}})
	}
	return set
}
