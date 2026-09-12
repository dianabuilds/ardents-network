package node

import (
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// closedBootstrapRecipient checks the entire locally observable hop before
// OPEN can dial. A direct source may reach only an adjacent duty; an interior
// must have an authenticated current adjacent Node in the same Role Domain.
// Neither caller-supplied addresses nor issuer responses select a recipient.
func closedBootstrapRecipient(config runtimeConfig, snapshot dutyFacts, receiver route.ClosedRoleReceiver, incomingKey [32]byte, open route.ClosedOpen, now time.Time) error {
	if config.CurrentClosedRoute == nil || snapshot.DeclaredFamily == "" || int(snapshot.CandidateCount) > len(snapshot.Candidates) {
		return errors.New("closed bootstrap current route is unavailable")
	}
	view, available := config.CurrentClosedRoute()
	if !available || int(view.NodeCount) > len(view.Nodes) || !closedRouteProfileMatchesSnapshot(view.Profile, snapshot, now) || view.Profile.Digest != receiver.ProfileDigest ||
		view.Profile.StateGeneration != receiver.StateGeneration || receiver.NodeID != snapshot.NodeID || receiver.DutyGeneration != snapshot.RecordGeneration {
		return errors.New("closed bootstrap receiver changed")
	}
	localFamily := sha256.Sum256([]byte(snapshot.DeclaredFamily))
	var previousFamily [32]byte
	if receiver.Subrole == 1 {
		if incomingKey != [32]byte{} {
			return errors.New("closed bootstrap adjacent source is invalid")
		}
	} else if receiver.Subrole == 2 {
		var matched bool
		for index := uint8(0); index < snapshot.CandidateCount; index++ {
			peer := snapshot.Candidates[index]
			if peer.PublicKey != incomingKey || incomingKey == [32]byte{} {
				continue
			}
			role, found := closedBootstrapRole(view, peer.NodeID)
			if matched || !found || role.RoleDomain != receiver.RoleDomain || role.Subrole != 1 || role.RecordDigest != peer.RecordDigest ||
				peer.NodeID == receiver.NodeID || peer.FamilyID == [32]byte{} || peer.FamilyID == localFamily || now.Before(peer.ValidFrom) ||
				!now.Before(peer.ValidUntil) || !now.Before(peer.AssignmentNotAfter) {
				return errors.New("closed bootstrap previous adjacency is unavailable")
			}
			matched, previousFamily = true, peer.FamilyID
		}
		if !matched {
			return errors.New("closed bootstrap authenticated adjacency is required")
		}
	} else {
		return errors.New("closed bootstrap receiver role is unavailable")
	}
	candidate, err := closedForwardRecipient(config, snapshot, open, now)
	if err != nil {
		return err
	}
	next, found := closedBootstrapRole(view, candidate.NodeID)
	if !found || candidate.NodeID == receiver.NodeID || candidate.FamilyID == [32]byte{} || candidate.FamilyID == localFamily ||
		candidate.FamilyID == previousFamily || now.Before(candidate.ValidFrom) || open.Deadline.After(candidate.ValidUntil) || open.Deadline.After(candidate.AssignmentNotAfter) {
		return errors.New("closed bootstrap recipient conflicts")
	}
	if receiver.Subrole == 1 && next.RoleDomain == receiver.RoleDomain && next.Subrole == 2 && open.Purpose == route.ClosedPurposeForwarding {
		return nil
	}
	if receiver.Subrole == 2 && next.RoleDomain == 2 && next.Subrole == 6 && open.Purpose == route.ClosedPurposeIssuer &&
		next.NodeID == view.Profile.IssuerNodeID && next.DutyGeneration == view.Profile.IssuerDutyGeneration {
		return nil
	}
	return errors.New("closed bootstrap private or alternate recipient is unavailable")
}

func closedBootstrapRole(view state.ClosedRouteView, id [32]byte) (state.ClosedRouteNodeView, bool) {
	var result state.ClosedRouteNodeView
	found := false
	if int(view.NodeCount) > len(view.Nodes) {
		return result, false
	}
	for index := uint8(0); index < view.NodeCount; index++ {
		if view.Nodes[index].NodeID == id {
			if found {
				return state.ClosedRouteNodeView{}, false
			}
			result, found = view.Nodes[index], true
		}
	}
	return result, found
}

func (server *closedForwardingServer) admitBootstrap(receiver route.ClosedRoleReceiver, incomingKey [32]byte, hello route.ClosedHello, helloSize int, frame route.ClosedLaneFrame) (*route.ClosedForwardingChannel, time.Time, error) {
	if receiver.Subrole != 1 && receiver.Subrole != 2 || receiver.Subrole == 1 && incomingKey != [32]byte{} {
		return nil, time.Time{}, errors.New("closed bootstrap receiving adjacency is unavailable")
	}
	issuer, err := route.DecodeClosedBootstrap(frame.Body)
	if err != nil || !issuer || frame.Kind != 3 || frame.Lane != 0 || hello == (route.ClosedHello{}) {
		return nil, time.Time{}, errors.New("closed bootstrap issuer operation is required")
	}
	adjacency := incomingKey
	if adjacency == [32]byte{} {
		adjacency[0] = 1
	}
	deadline := server.clock().UTC().Add(10 * time.Second)
	if hello.Deadline.Before(deadline) {
		deadline = hello.Deadline
	}
	lease, err := server.bootstrap.Admit(adjacency, deadline)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer lease.Release()
	if err := lease.Receive(uint64(helloSize + 16 + len(frame.Body))); err != nil {
		return nil, time.Time{}, err
	}
	channel, err := route.NewClosedBootstrapForwardingChannel(lease, server.limits, func(open route.ClosedOpen) error {
		current, err := currentFacts(server.config)
		if err != nil {
			return err
		}
		return closedBootstrapRecipient(server.config, current, receiver, incomingKey, open, server.clock().UTC())
	}, server.clock)
	return channel, deadline, err
}
