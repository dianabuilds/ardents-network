package route

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func TestClosedForwardingPreservesAdjacentRoleDomains(t *testing.T) {
	for _, domain := range []uint8{1, 3, 4} {
		for _, subrole := range []uint8{1, 2} {
			if !ClosedPurposePermitsDuty(ardp.PurposeForwarding, domain, subrole) {
				t.Errorf("selected adjacent domain %d/%d refused", domain, subrole)
			}
		}
	}
	for _, subrole := range []uint8{1, 2} {
		if ClosedPurposePermitsDuty(ardp.PurposeForwarding, 2, subrole) {
			t.Errorf("non-adjacent Rendezvous admitted for %d", subrole)
		}
	}
	if !ClosedPurposePermitsDuty(ardp.PurposeIntroduction, 4, 3) || !ClosedPurposePermitsDuty(ardp.PurposeDataJoin, 2, 4) {
		t.Fatal("terminal duties changed")
	}
}
