//go:build linux

package route

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func TestClosedPurposeAssignmentTablePermitsOnlyNormativeDuties(t *testing.T) {
	cases := []struct {
		purpose         ardp.Purpose
		domain, subrole uint8
		allowed         bool
	}{
		{ardp.PurposeIssuer, closedRoleDomainRendezvous, closedDutyIssuance, true},
		{ardp.PurposeName, closedRoleDomainRendezvous, closedDutyResolution, true},
		{ardp.PurposeReachability, closedRoleDomainRendezvous, closedDutyResolution, true},
		{ardp.PurposeIntroduction, closedRoleDomainIntroduction, closedDutyIntroduction, true},
		{ardp.PurposeSubmission, closedRoleDomainIntroduction, closedDutyIntroduction, true},
		{ardp.PurposeDataJoin, closedRoleDomainRendezvous, closedDutyDataJoin, true},
		{ardp.PurposeForwarding, closedRoleDomainInitiator, closedDutyAdjacent, true},
		{ardp.PurposeForwarding, closedRoleDomainResponder, closedDutyInterior, true},
		{ardp.PurposeIssuer, closedRoleDomainRendezvous, closedDutyResolution, false},
		{ardp.PurposeDataJoin, closedRoleDomainIntroduction, closedDutyIntroduction, false},
		{ardp.PurposeForwarding, closedRoleDomainIntroduction, closedDutyIntroduction, false},
	}
	for _, test := range cases {
		if got := ClosedPurposePermitsDuty(test.purpose, test.domain, test.subrole); got != test.allowed {
			t.Fatalf("purpose %d with %d/%d allowed=%t, want %t", test.purpose, test.domain, test.subrole, got, test.allowed)
		}
	}
}
