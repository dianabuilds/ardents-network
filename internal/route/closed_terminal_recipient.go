//go:build linux

package route

import (
	"encoding/hex"
	"errors"
	"time"
)

func (prefix *ClosedSourcePrefix) terminalPeer(purpose ClosedPurpose) (closedBootstrapPeer, error) {
	if prefix == nil || prefix.channels == nil {
		return closedBootstrapPeer{}, errors.New("closed resolution prefix unavailable")
	}
	return closedTerminalPeer(prefix.source, prefix.selection, prefix.plan, purpose)
}

// Resolve only current public duty facts; this function creates no channels.
func closedTerminalPeer(source ClosedBootstrapState, selection ClosedBootstrapSelection, plan closedBootstrapPlan, purpose ClosedPurpose) (closedBootstrapPeer, error) {
	if purpose != ClosedPurposeReachability && purpose != ClosedPurposeIntroduction && purpose != ClosedPurposeSubmission && purpose != ClosedPurposeDataJoin {
		return closedBootstrapPeer{}, errors.New("closed terminal purpose unavailable")
	}
	if err := plan.current(source, selection); err != nil {
		return closedBootstrapPeer{}, err
	}
	view, err := source.CurrentClosedRoute()
	if err != nil || view.Profile != plan.profile || int(view.NodeCount) > len(view.Nodes) {
		return closedBootstrapPeer{}, errors.New("closed resolution profile changed")
	}
	snapshot, err := source.Current()
	if err != nil || snapshot.NetworkID != view.Profile.NetworkID || snapshot.Generation != hex.EncodeToString(view.Profile.StateGeneration[:]) || snapshot.Epoch != view.Profile.Epoch || snapshot.Profile != ClosedRouteProfile || snapshot.Digest != view.Profile.StateDigest || snapshot.Freshness != "fresh" || snapshot.Conflicting || int(snapshot.CandidateCount) > len(snapshot.Candidates) {
		return closedBootstrapPeer{}, errors.New("closed resolution State unavailable")
	}
	now := time.Now().UTC()
	if now.Before(snapshot.EpochValidFrom) || !now.Before(snapshot.ValidUntil) {
		return closedBootstrapPeer{}, errors.New("closed resolution State expired")
	}
	var selected closedBootstrapPeer
	found := false
	for _, role := range view.Nodes[:view.NodeCount] {
		if !ClosedPurposePermitsDuty(purpose, role.RoleDomain, role.Subrole) {
			continue
		}
		if found || role.NodeID == [32]byte{} || role.DutyGeneration == 0 {
			return closedBootstrapPeer{}, errors.New("closed resolution duty ambiguous")
		}
		found = true
		for _, candidate := range snapshot.Candidates[:snapshot.CandidateCount] {
			if candidate.NodeID != role.NodeID {
				continue
			}
			if selected.node != [32]byte{} || candidate.RecordDigest != role.RecordDigest || candidate.PublicKey == [32]byte{} || candidate.FamilyID == [32]byte{} ||
				candidate.Capacity == 0 || now.Before(candidate.ValidFrom) || !now.Before(candidate.ValidUntil) || !now.Before(candidate.AssignmentNotAfter) ||
				!literalEndpoint(candidate.Endpoint) || CarrierProfile(candidate.CarrierProfile) != ClosedCarrierTCP && CarrierProfile(candidate.CarrierProfile) != ClosedCarrierQUIC {
				return closedBootstrapPeer{}, errors.New("closed resolution Node Record unavailable")
			}
			selected = closedBootstrapPeer{node: role.NodeID, key: candidate.PublicKey, family: candidate.FamilyID, record: candidate.RecordDigest,
				generation: role.DutyGeneration, endpoint: candidate.Endpoint, carrier: CarrierProfile(candidate.CarrierProfile), notAfter: candidate.ValidUntil}
			if candidate.AssignmentNotAfter.Before(selected.notAfter) {
				selected.notAfter = candidate.AssignmentNotAfter
			}
		}
	}
	if !found || selected.node == [32]byte{} {
		return closedBootstrapPeer{}, errors.New("closed resolution duty absent")
	}
	for _, adjacent := range plan.peers[:2] {
		if adjacent.node == selected.node || adjacent.key == selected.key || adjacent.family == selected.family {
			return closedBootstrapPeer{}, errors.New("closed resolution conflicts with source")
		}
	}
	return selected, nil
}

// DataJoinRecipient exposes only independently selected current duty facts.
// The Source or Responder prefix still owns the future dial and family checks;
// a capsule cannot provide a literal endpoint or select a different duty.
func (prefix *ClosedSourcePrefix) DataJoinRecipient() ([32]byte, uint64, time.Time, error) {
	if prefix == nil || prefix.plan.domain != 1 && prefix.plan.domain != 3 {
		return [32]byte{}, 0, time.Time{}, errors.New("closed data join requires Source or Responder ownership")
	}
	if prefix.channels == nil {
		return [32]byte{}, 0, time.Time{}, errors.New("closed resolution prefix unavailable")
	}
	return closedDataJoinRecipient(prefix.source, prefix.selection, prefix.plan)
}

func closedDataJoinRecipient(source ClosedBootstrapState, selection ClosedBootstrapSelection, plan closedBootstrapPlan) ([32]byte, uint64, time.Time, error) {
	peer, err := closedTerminalPeer(source, selection, plan, ClosedPurposeDataJoin)
	if err != nil {
		return [32]byte{}, 0, time.Time{}, err
	}
	controls := []closedBootstrapPeer{plan.peers[2]}
	for _, purpose := range []ClosedPurpose{ClosedPurposeReachability, ClosedPurposeIntroduction} {
		control, err := closedTerminalPeer(source, selection, plan, purpose)
		if err != nil {
			return [32]byte{}, 0, time.Time{}, err
		}
		controls = append(controls, control)
	}
	for _, control := range controls {
		if peer.node == control.node || peer.key == control.key || peer.family == control.family {
			return [32]byte{}, 0, time.Time{}, errors.New("closed data join conflicts with control duty")
		}
	}
	end := peer.notAfter
	if plan.deadline.Before(end) {
		end = plan.deadline
	}
	return peer.node, peer.generation, end, err
}
