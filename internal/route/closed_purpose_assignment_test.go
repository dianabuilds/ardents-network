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
		{ardp.PurposeIntroduction, ClosedRoleDomainIntroduction, closedDutyIntroduction, true},
		{ardp.PurposeSubmission, ClosedRoleDomainIntroduction, closedDutyIntroduction, true},
		{ardp.PurposeDataJoin, closedRoleDomainRendezvous, closedDutyDataJoin, true},
		{ardp.PurposeForwarding, ClosedRoleDomainInitiator, ClosedDutyAdjacent, true},
		{ardp.PurposeForwarding, closedRoleDomainResponder, closedDutyInterior, true},
		{ardp.PurposeIssuer, closedRoleDomainRendezvous, closedDutyResolution, false},
		{ardp.PurposeDataJoin, ClosedRoleDomainIntroduction, closedDutyIntroduction, false},
		{ardp.PurposeForwarding, ClosedRoleDomainIntroduction, closedDutyIntroduction, false},
	}
	for _, test := range cases {
		if got := ClosedPurposePermitsDuty(test.purpose, test.domain, test.subrole); got != test.allowed {
			t.Fatalf("purpose %d with %d/%d allowed=%t, want %t", test.purpose, test.domain, test.subrole, got, test.allowed)
		}
	}
}
