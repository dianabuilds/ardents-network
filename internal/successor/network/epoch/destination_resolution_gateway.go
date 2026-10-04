package epoch

import "errors"

const MaxDestinationResolutionProfileBytes = maximumDestinationResolutionProfileBytes

const DomainDestinationResolution = destinationResolutionDomain

const (
	destinationResolutionDomain              = "destination-resolution"
	maximumDestinationResolutionProfileBytes = 4096
)

// attachDestinationResolutionGateway binds the v2 Epoch's one explicitly
// selected Gateway profile to the candidate State assigned to that duty. The
// profile bytes remain opaque here: Reachability owns their self-signature and
// OHTTP grammar.
func attachDestinationResolutionGateway(decision *Decision) error {
	if decision == nil || decision.epoch.version < 2 {
		return nil
	}
	if decision.epoch.profile != interactiveRouteProfile {
		if decision.epoch.destinationResolutionNodeID != [32]byte{} || len(decision.epoch.destinationResolutionProfile) != 0 {
			return errors.New("non-Route Epoch carries a Destination Resolution Gateway")
		}
		return nil
	}
	if decision.epoch.destinationResolutionNodeID == [32]byte{} || len(decision.epoch.destinationResolutionProfile) == 0 ||
		len(decision.epoch.destinationResolutionProfile) > maximumDestinationResolutionProfileBytes {
		return errors.New("interactive Route Epoch lacks a Destination Resolution Gateway profile")
	}
	selected := -1
	for index, candidate := range decision.Candidates {
		if candidate.Domain != destinationResolutionDomain {
			continue
		}
		if selected >= 0 || candidate.NodeID == [32]byte{} || candidate.FamilyID == [32]byte{} {
			return errors.New("destination resolution gateway state assignment is ambiguous")
		}
		selected = index
	}
	if selected < 0 || decision.Candidates[selected].NodeID != decision.epoch.destinationResolutionNodeID {
		return errors.New("destination resolution gateway profile does not match its state assignment")
	}
	decision.Snapshot.DestinationResolutionNodeID = decision.Candidates[selected].NodeID
	copy(decision.Snapshot.DestinationResolutionProfile[:], decision.epoch.destinationResolutionProfile)
	decision.Snapshot.DestinationResolutionProfileSize = uint16(len(decision.epoch.destinationResolutionProfile))
	return nil
}
