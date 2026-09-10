//go:build linux

package route

import (
	"encoding/hex"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// ClosedBootstrapState is the opened State owner's live public projection.
// Applications cannot provide a profile, transport address, key or validity
// switch. Selection only names members already retained by Endpoint's sets.
type ClosedBootstrapState interface {
	Current() (state.Snapshot, error)
	CurrentClosedRoute() (state.ClosedRouteView, error)
}

// ClosedBootstrapSelection identifies the active members of the owner's
// retained Entry/Interior sets. It does not admit resampling, retries or a
// caller-selected issuer. ProfileDigest binds an outstanding issuance request.
type ClosedBootstrapSelection struct {
	ProfileDigest, EntryNodeID, InteriorNodeID [32]byte
}

type closedBootstrapPeer struct {
	node, key, family, record [32]byte
	generation                uint64
	endpoint                  string
	carrier                   CarrierProfile
	notAfter                  time.Time
}

type closedBootstrapPlan struct {
	domain   uint8
	profile  state.ClosedProfileView
	peers    [3]closedBootstrapPeer
	deadline time.Time
}

func prepareClosedBootstrap(source ClosedBootstrapState, selection ClosedBootstrapSelection, now time.Time) (closedBootstrapPlan, error) {
	return prepareClosedPrefix(source, selection, 1, now)
}

func prepareClosedPrefix(source ClosedBootstrapState, selection ClosedBootstrapSelection, adjacentDomain uint8, now time.Time) (closedBootstrapPlan, error) {
	if !ClosedPurposePermitsDuty(ClosedPurposeForwarding, adjacentDomain, closedDutyAdjacent) {
		return closedBootstrapPlan{}, errors.New("closed prefix adjacent domain unavailable")
	}
	if source == nil || selection.ProfileDigest == [32]byte{} || selection.EntryNodeID == [32]byte{} || selection.InteriorNodeID == [32]byte{} {
		return closedBootstrapPlan{}, errors.New("closed bootstrap selection is unavailable")
	}
	view, err := source.CurrentClosedRoute()
	if err != nil {
		return closedBootstrapPlan{}, err
	}
	snapshot, err := source.Current()
	if err != nil {
		return closedBootstrapPlan{}, err
	}
	profile := view.Profile
	if snapshot.Profile != ClosedRouteProfile || snapshot.Freshness != "fresh" || snapshot.Conflicting ||
		snapshot.NetworkID != profile.NetworkID || snapshot.Digest != profile.StateDigest || snapshot.Epoch != profile.Epoch ||
		snapshot.Generation != hex.EncodeToString(profile.StateGeneration[:]) || profile.Digest != selection.ProfileDigest ||
		now.Before(profile.NotBefore) || !now.Before(profile.NotAfter) || now.Before(snapshot.EpochValidFrom) || !now.Before(snapshot.ValidUntil) ||
		view.NodeCount == 0 || int(view.NodeCount) > len(view.Nodes) || int(snapshot.CandidateCount) > len(snapshot.Candidates) {
		return closedBootstrapPlan{}, errors.New("closed bootstrap State is unavailable")
	}
	plan := closedBootstrapPlan{domain: adjacentDomain, profile: profile, deadline: now.Add(10 * time.Second).Truncate(time.Second)}
	for _, limit := range []time.Time{profile.NotAfter, snapshot.ValidUntil} {
		if limit.Before(plan.deadline) {
			plan.deadline = limit.Truncate(time.Second)
		}
	}
	ids := [3][32]byte{selection.EntryNodeID, selection.InteriorNodeID, profile.IssuerNodeID}
	for index, id := range ids {
		var role state.ClosedRouteNodeView
		found := false
		for _, candidate := range view.Nodes[:view.NodeCount] {
			if candidate.NodeID == id {
				if found {
					return closedBootstrapPlan{}, errors.New("closed bootstrap recipient is ambiguous")
				}
				role, found = candidate, true
			}
		}
		domain, subrole := adjacentDomain, uint8(index+1)
		if index == 2 {
			domain, subrole = 2, 6
		}
		if !found || id == [32]byte{} || role.RoleDomain != domain || role.Subrole != subrole || role.DutyGeneration == 0 ||
			index == 2 && role.DutyGeneration != profile.IssuerDutyGeneration {
			return closedBootstrapPlan{}, errors.New("closed bootstrap recipient role is unavailable")
		}
		found = false
		for _, candidate := range snapshot.Candidates[:snapshot.CandidateCount] {
			if candidate.NodeID != id {
				continue
			}
			if found || candidate.RecordDigest != role.RecordDigest || candidate.PublicKey == [32]byte{} || candidate.FamilyID == [32]byte{} ||
				candidate.Capacity == 0 || now.Before(candidate.ValidFrom) || !now.Before(candidate.ValidUntil) || !now.Before(candidate.AssignmentNotAfter) ||
				!literalEndpoint(candidate.Endpoint) || (CarrierProfile(candidate.CarrierProfile) != ClosedCarrierTCP && CarrierProfile(candidate.CarrierProfile) != ClosedCarrierQUIC) {
				return closedBootstrapPlan{}, errors.New("closed bootstrap Node Record is unavailable")
			}
			found = true
			peer := closedBootstrapPeer{node: id, key: candidate.PublicKey, family: candidate.FamilyID, record: candidate.RecordDigest,
				generation: role.DutyGeneration, endpoint: candidate.Endpoint, carrier: CarrierProfile(candidate.CarrierProfile), notAfter: candidate.ValidUntil}
			if candidate.AssignmentNotAfter.Before(peer.notAfter) {
				peer.notAfter = candidate.AssignmentNotAfter
			}
			plan.peers[index] = peer
			if peer.notAfter.Before(plan.deadline) {
				plan.deadline = peer.notAfter.Truncate(time.Second)
			}
		}
		if !found {
			return closedBootstrapPlan{}, errors.New("closed bootstrap Node Record is absent")
		}
		for previous := 0; previous < index; previous++ {
			if plan.peers[previous].node == id || plan.peers[previous].key == plan.peers[index].key || plan.peers[previous].family == plan.peers[index].family {
				return closedBootstrapPlan{}, errors.New("closed bootstrap roles conflict")
			}
		}
	}
	if !now.Before(plan.deadline) {
		return closedBootstrapPlan{}, errors.New("closed bootstrap window is exhausted")
	}
	return plan, nil
}

func (plan closedBootstrapPlan) current(source ClosedBootstrapState, selection ClosedBootstrapSelection) error {
	next, err := prepareClosedPrefix(source, selection, plan.domain, time.Now().UTC())
	if err != nil {
		return err
	}
	if next.profile != plan.profile || next.peers != plan.peers || !time.Now().Before(plan.deadline) {
		return errors.New("closed bootstrap selection changed")
	}
	return nil
}
