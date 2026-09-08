package node

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// closedRouteReceiver projects one local recipient from the exact State
// accepted closed profile. It never accepts a plan-supplied digest, role or
// duty, and a State successor makes the receiver unavailable before dial.
func closedRouteReceiver(config runtimeConfig, snapshot dutyFacts, purpose route.ClosedPurpose, now time.Time) (route.ClosedRoleReceiver, bool) {
	if config.CurrentClosedRoute == nil || snapshot.Profile != route.ClosedRouteProfile || !snapshot.Fresh || snapshot.Conflicting ||
		snapshot.NodeID == [32]byte{} || snapshot.RecordGeneration == 0 || !now.Before(snapshot.ValidUntil) || !now.Before(snapshot.RecordValidUntil) {
		return route.ClosedRoleReceiver{}, false
	}
	view, available := config.CurrentClosedRoute()
	if !available || !closedRouteProfileMatchesSnapshot(view.Profile, snapshot, now) {
		return route.ClosedRoleReceiver{}, false
	}
	var recipient state.ClosedRouteNodeView
	found := false
	for index := uint8(0); index < view.NodeCount; index++ {
		node := view.Nodes[index]
		if node.NodeID != snapshot.NodeID {
			continue
		}
		if found || node.RecordDigest == [32]byte{} || node.DutyGeneration != snapshot.RecordGeneration {
			return route.ClosedRoleReceiver{}, false
		}
		recipient, found = node, true
	}
	if !found || !route.ClosedPurposePermitsDuty(purpose, recipient.RoleDomain, recipient.Subrole) {
		return route.ClosedRoleReceiver{}, false
	}
	return route.ClosedRoleReceiver{NetworkID: view.Profile.NetworkID, StateGeneration: view.Profile.StateGeneration,
		StateDigest: view.Profile.StateDigest, ProfileDigest: view.Profile.Digest, NodeID: recipient.NodeID,
		RecordDigest: recipient.RecordDigest, DutyGeneration: recipient.DutyGeneration, RoleDomain: recipient.RoleDomain,
		Subrole: recipient.Subrole, ExpectedPurpose: purpose, NotAfter: view.Profile.NotAfter}, true
}

func closedRouteProfileMatchesSnapshot(profile state.ClosedProfileView, snapshot dutyFacts, now time.Time) bool {
	return profile.NetworkID == snapshot.NetworkID && profile.StateDigest == snapshot.Digest && profile.Epoch == snapshot.Epoch &&
		profile.Digest != [32]byte{} && !profile.NotBefore.After(now) && now.Before(profile.NotAfter) &&
		profile.NotBefore.Before(snapshot.EpochValidFrom) == false && !profile.NotAfter.After(snapshot.ValidUntil) &&
		!profile.NotAfter.After(snapshot.RecordValidUntil) && closedStateGenerationMatches(profile.StateGeneration, snapshot.Generation)
}
