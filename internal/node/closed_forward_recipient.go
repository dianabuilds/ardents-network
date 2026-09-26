package node

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// closedForwardRecipient intersects an OPEN with current State's signed
// closed-route recipient and its exact public Node Record. It is deliberately
// a pre-dial check: the returned record is not a Carrier and cannot select a
// fallback peer.
func closedForwardRecipient(config runtimeConfig, snapshot state.NodeDuty, open route.ClosedOpen, now time.Time) (state.NodeDutyCandidate, error) {
	if config.CurrentClosedRoute == nil || !now.Before(open.Deadline) || snapshot.Profile != route.ClosedRouteProfile || !snapshot.Fresh || snapshot.Conflicting {
		return state.NodeDutyCandidate{}, errors.New("closed forwarding recipient is unavailable")
	}
	view, err := config.CurrentClosedRoute()
	if err != nil || !closedRouteProfileMatchesSnapshot(view.Profile, snapshot, now) {
		return state.NodeDutyCandidate{}, errors.New("closed forwarding recipient is unavailable")
	}
	var recipient state.ClosedRouteNodeView
	matchedRecipient := false
	for index := uint8(0); index < view.NodeCount; index++ {
		node := view.Nodes[index]
		if node.NodeID != open.NextNodeID {
			continue
		}
		if matchedRecipient || node.DutyGeneration != open.NextDutyGeneration || !route.ClosedPurposePermitsDuty(open.Purpose, node.RoleDomain, node.Subrole) {
			return state.NodeDutyCandidate{}, errors.New("closed forwarding recipient is unavailable")
		}
		recipient, matchedRecipient = node, true
	}
	if !matchedRecipient {
		return state.NodeDutyCandidate{}, errors.New("closed forwarding recipient is unavailable")
	}
	var candidate state.NodeDutyCandidate
	matchedCandidate := false
	for index := uint8(0); index < snapshot.CandidateCount; index++ {
		value := snapshot.Candidates[index]
		if value.NodeID != recipient.NodeID {
			continue
		}
		if matchedCandidate || value.RecordDigest != recipient.RecordDigest || value.PublicKey == [32]byte{} || !literalNodeEndpoint(value.Endpoint) ||
			(route.CarrierProfile(value.CarrierProfile) != route.ClosedCarrierTCP && route.CarrierProfile(value.CarrierProfile) != route.ClosedCarrierQUIC) ||
			!now.Before(value.ValidUntil) || !now.Before(value.AssignmentNotAfter) {
			return state.NodeDutyCandidate{}, errors.New("closed forwarding recipient is unavailable")
		}
		candidate, matchedCandidate = value, true
	}
	if !matchedCandidate {
		return state.NodeDutyCandidate{}, errors.New("closed forwarding recipient is unavailable")
	}
	return candidate, nil
}
