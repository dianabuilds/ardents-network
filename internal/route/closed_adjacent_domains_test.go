package route

import "testing"

func TestClosedForwardingPreservesAdjacentRoleDomains(t *testing.T) {
	for _, domain := range []uint8{1, 3, 4} {
		for _, subrole := range []uint8{1, 2} {
			if !ClosedPurposePermitsDuty(ClosedPurposeForwarding, domain, subrole) {
				t.Errorf("selected adjacent domain %d/%d refused", domain, subrole)
			}
		}
	}
	for _, subrole := range []uint8{1, 2} {
		if ClosedPurposePermitsDuty(ClosedPurposeForwarding, 2, subrole) {
			t.Errorf("non-adjacent Rendezvous admitted for %d", subrole)
		}
	}
	if !ClosedPurposePermitsDuty(ClosedPurposeIntroduction, 4, 3) || !ClosedPurposePermitsDuty(ClosedPurposeDataJoin, 2, 4) {
		t.Fatal("terminal duties changed")
	}
}
