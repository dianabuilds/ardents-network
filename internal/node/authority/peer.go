package authority

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// PeerCurrent accepts one exact current State peer key for a shared Carrier.
func (source Source) PeerCurrent(snapshot state.NodeDuty, key [32]byte, now time.Time) bool {
	if key == [32]byte{} || source.CurrentRoute == nil || snapshot.Profile != carrier.ClosedRouteProfile || !snapshot.Fresh || snapshot.Conflicting {
		return false
	}
	view, err := source.CurrentRoute()
	if err != nil || !ProfileMatchesSnapshot(view.Profile, snapshot, now) {
		return false
	}
	matched := false
	for index := uint8(0); index < view.NodeCount; index++ {
		recipient := view.Nodes[index]
		if recipient.NodeID == [32]byte{} || recipient.NodeID == snapshot.NodeID || recipient.DutyGeneration == 0 || !route.ClosedPurposePermitsDuty(ardp.PurposeForwarding, recipient.RoleDomain, recipient.Subrole) {
			continue
		}
		for peerIndex := uint8(0); peerIndex < snapshot.CandidateCount; peerIndex++ {
			peer := snapshot.Candidates[peerIndex]
			if peer.NodeID != recipient.NodeID || peer.PublicKey != key || peer.PublicKey == [32]byte{} || peer.RecordDigest != recipient.RecordDigest || !now.Before(peer.ValidUntil) || !now.Before(peer.AssignmentNotAfter) {
				continue
			}
			if matched {
				return false
			}
			matched = true
		}
	}
	return matched
}
