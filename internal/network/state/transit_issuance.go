package state

import "errors"

const (
	transitIssuanceDomain              = "transit-issuance"
	maximumTransitIssuanceProfileBytes = 4096
)

// attachTransitIssuanceDuty binds the v3 Epoch's one selected membership
// Transit Grant issuer profile to its assigned candidate. State does not parse
// the OHTTP/profile grammar; the credential package owns that verification.
func attachTransitIssuanceDuty(decision *verifiedEpochDecision) error {
	if decision == nil || decision.epoch.version < 3 {
		return nil
	}
	if decision.epoch.profile != interactiveRouteProfile {
		if decision.epoch.transitIssuanceNodeID != [32]byte{} || len(decision.epoch.transitIssuanceProfile) != 0 {
			return errors.New("non-Route Epoch carries a transit issuance duty")
		}
		return nil
	}
	if decision.epoch.transitIssuanceNodeID == [32]byte{} || len(decision.epoch.transitIssuanceProfile) == 0 ||
		len(decision.epoch.transitIssuanceProfile) > maximumTransitIssuanceProfileBytes {
		return errors.New("interactive Route Epoch lacks a transit issuance profile")
	}
	selected := -1
	for index, candidate := range decision.Candidates {
		if candidate.Domain != transitIssuanceDomain {
			continue
		}
		if selected >= 0 || candidate.NodeID == [32]byte{} || candidate.FamilyID == [32]byte{} {
			return errors.New("transit issuance State assignment is ambiguous")
		}
		selected = index
	}
	if selected < 0 || decision.Candidates[selected].NodeID != decision.epoch.transitIssuanceNodeID {
		return errors.New("transit issuance profile does not match its State assignment")
	}
	decision.Snapshot.TransitIssuanceNodeID = decision.Candidates[selected].NodeID
	copy(decision.Snapshot.TransitIssuanceProfile[:], decision.epoch.transitIssuanceProfile)
	decision.Snapshot.TransitIssuanceProfileSize = uint16(len(decision.epoch.transitIssuanceProfile))
	return nil
}
