package route

import "github.com/dianabuilds/ardents-network/internal/route/ardp"

const (
	closedRoleDomainInitiator    = uint8(1)
	closedRoleDomainRendezvous   = uint8(2)
	closedRoleDomainResponder    = uint8(3)
	closedRoleDomainIntroduction = uint8(4)

	closedDutyAdjacent     = uint8(1)
	closedDutyInterior     = uint8(2)
	closedDutyIntroduction = uint8(3)
	closedDutyDataJoin     = uint8(4)
	closedDutyResolution   = uint8(5)
	closedDutyIssuance     = uint8(6)
)

// ClosedPurposePermitsDuty implements the generation-3 normative assignment
// table. It is only one admission predicate: callers must still enforce the
// accepted profile/Node Record, role transitions, family exclusions, exact
// duty and all admission and resource limits before they dial or forward.
func ClosedPurposePermitsDuty(purpose ardp.Purpose, domain, subrole uint8) bool {
	switch purpose {
	case ardp.PurposeIssuer:
		return domain == closedRoleDomainRendezvous && subrole == closedDutyIssuance
	case ardp.PurposeName, ardp.PurposeReachability:
		return domain == closedRoleDomainRendezvous && subrole == closedDutyResolution
	case ardp.PurposeIntroduction, ardp.PurposeSubmission:
		return domain == closedRoleDomainIntroduction && subrole == closedDutyIntroduction
	case ardp.PurposeDataJoin:
		return domain == closedRoleDomainRendezvous && subrole == closedDutyDataJoin
	case ardp.PurposeForwarding:
		return (domain == closedRoleDomainInitiator || domain == closedRoleDomainResponder || domain == closedRoleDomainIntroduction) &&
			(subrole == closedDutyAdjacent || subrole == closedDutyInterior)
	default:
		return false
	}
}

func validClosedDutyAssignment(domain, subrole uint8) bool {
	return ClosedPurposePermitsDuty(ardp.PurposeIssuer, domain, subrole) ||
		ClosedPurposePermitsDuty(ardp.PurposeName, domain, subrole) ||
		ClosedPurposePermitsDuty(ardp.PurposeIntroduction, domain, subrole) ||
		ClosedPurposePermitsDuty(ardp.PurposeDataJoin, domain, subrole) ||
		ClosedPurposePermitsDuty(ardp.PurposeForwarding, domain, subrole)
}
