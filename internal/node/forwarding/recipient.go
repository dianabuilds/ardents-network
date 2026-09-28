package forwarding

import (
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// recipient intersects an OPEN with current State's signed
// closed-route recipient and its exact public Node Record. It is deliberately
// a pre-dial check: the returned record is not a Carrier and cannot select a
// fallback peer.
func recipient(source authority.Source, snapshot state.NodeDuty, open route.ClosedOpen, now time.Time, literalEndpoint func(string) bool) (state.NodeDutyCandidate, error) {
	if source.CurrentRoute == nil || !now.Before(open.Deadline) || snapshot.Profile != carrier.ClosedRouteProfile || !snapshot.Fresh || snapshot.Conflicting {
		return state.NodeDutyCandidate{}, errors.New("closed forwarding recipient is unavailable")
	}
	view, err := source.CurrentRoute()
	if err != nil || !authority.ProfileMatchesSnapshot(view.Profile, snapshot, now) {
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
		if matchedCandidate || value.RecordDigest != recipient.RecordDigest || value.PublicKey == [32]byte{} || !literalEndpoint(value.Endpoint) ||
			(carrier.CarrierProfile(value.CarrierProfile) != carrier.ClosedCarrierTCP && carrier.CarrierProfile(value.CarrierProfile) != carrier.ClosedCarrierQUIC) ||
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

// dialAddress retains the State-selected recipient while routing
// this Node's physical Carrier through one operator-owned transparent relay.
// Authentication still uses the selected recipient key after the relay dial.
func dialAddress(advertised, relay string) (string, error) {
	if relay == "" {
		return advertised, nil
	}
	if !ValidCarrierEndpoint(advertised) || !ValidCarrierEndpoint(relay) {
		return "", errors.New("closed forwarding Carrier relay endpoint is invalid")
	}
	return relay, nil
}

// ValidCarrierEndpoint requires a literal, specified IP and nonzero port.
func ValidCarrierEndpoint(endpoint string) bool {
	host, portText, err := net.SplitHostPort(endpoint)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	port, err := strconv.ParseUint(portText, 10, 16)
	return err == nil && port != 0 && ip != nil && !ip.IsUnspecified()
}
