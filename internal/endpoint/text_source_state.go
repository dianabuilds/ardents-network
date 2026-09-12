//go:build linux

package endpoint

import (
	"encoding/hex"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type textRoleMember struct {
	entry.ClosedSetMember
	subrole uint8
}

// closedTextRoleMembers joins two live projections from the opened State
// owner. The worker and permission response cannot provide these bindings.
func (endpoint *endpoint) closedTextRoleMembers() (state.ClosedProfileView, []textRoleMember, time.Time, error) {
	source, ok := endpoint.closedState.(route.ClosedBootstrapState)
	if !ok || endpoint.clock == nil || endpoint.closedRoleRoot == "" {
		return state.ClosedProfileView{}, nil, time.Time{}, errors.New("text State owner unavailable")
	}
	now := endpoint.clock().UTC()
	view, err := source.CurrentClosedRoute()
	if err != nil {
		return state.ClosedProfileView{}, nil, now, err
	}
	snapshot, err := source.Current()
	profile := view.Profile
	if err != nil || profile.NetworkID != endpoint.network || snapshot.NetworkID != endpoint.network || snapshot.Profile != route.ClosedRouteProfile ||
		snapshot.Freshness != "fresh" || snapshot.Conflicting || snapshot.Digest != profile.StateDigest || snapshot.Epoch != profile.Epoch ||
		snapshot.Generation != hex.EncodeToString(profile.StateGeneration[:]) || now.Before(profile.NotBefore) || !now.Before(profile.NotAfter) ||
		now.Before(snapshot.EpochValidFrom) || !now.Before(snapshot.ValidUntil) || view.NodeCount == 0 || int(view.NodeCount) > len(view.Nodes) || int(snapshot.CandidateCount) > len(snapshot.Candidates) {
		return state.ClosedProfileView{}, nil, now, errors.New("text State binding unavailable")
	}
	issuerIndex := -1
	for index, candidate := range snapshot.Candidates[:snapshot.CandidateCount] {
		if candidate.NodeID == profile.IssuerNodeID {
			if issuerIndex >= 0 {
				return state.ClosedProfileView{}, nil, now, errors.New("text issuer ambiguous")
			}
			issuerIndex = index
		}
	}
	if issuerIndex < 0 {
		return state.ClosedProfileView{}, nil, now, errors.New("text issuer unavailable")
	}
	issuer := snapshot.Candidates[issuerIndex]
	if issuer.PublicKey == [32]byte{} || issuer.FamilyID == [32]byte{} {
		return state.ClosedProfileView{}, nil, now, errors.New("text issuer unavailable")
	}
	var members []textRoleMember
	seen := make(map[[32]byte]bool)
	for _, role := range view.Nodes[:view.NodeCount] {
		if seen[role.NodeID] {
			return state.ClosedProfileView{}, nil, now, errors.New("text role ambiguous")
		}
		seen[role.NodeID] = true
		if !route.ClosedPurposePermitsDuty(route.ClosedPurposeForwarding, role.RoleDomain, role.Subrole) {
			continue
		}
		found := false
		for _, candidate := range snapshot.Candidates[:snapshot.CandidateCount] {
			if candidate.NodeID != role.NodeID {
				continue
			}
			if found {
				return state.ClosedProfileView{}, nil, now, errors.New("text Node Record ambiguous")
			}
			found = true
			if candidate.RecordDigest != role.RecordDigest || role.DutyGeneration == 0 || candidate.PublicKey == [32]byte{} || candidate.FamilyID == [32]byte{} ||
				candidate.Capacity == 0 || now.Before(candidate.ValidFrom) || !now.Before(candidate.ValidUntil) || !now.Before(candidate.AssignmentNotAfter) ||
				candidate.CarrierProfile != string(route.ClosedCarrierTCP) && candidate.CarrierProfile != string(route.ClosedCarrierQUIC) {
				continue
			}
			if candidate.NodeID == issuer.NodeID || candidate.PublicKey == issuer.PublicKey || candidate.FamilyID == issuer.FamilyID {
				continue
			}
			conflict, err := duty.ReadConflict(endpoint.closedRoleRoot, endpoint.clock, candidate.NodeID, candidate.FamilyID)
			if err != nil {
				return state.ClosedProfileView{}, nil, now, err
			}
			if conflict {
				continue
			}
			until := candidate.ValidUntil
			if candidate.AssignmentNotAfter.Before(until) {
				until = candidate.AssignmentNotAfter
			}
			member := textRoleMember{ClosedSetMember: entry.ClosedSetMember{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, FamilyID: candidate.FamilyID,
				RecordDigest: candidate.RecordDigest, DutyGeneration: role.DutyGeneration, Domain: role.RoleDomain, NotAfter: until}, subrole: role.Subrole}
			if textRoleSeparated(member, view, snapshot, now) {
				members = append(members, member)
			}
		}
	}
	return profile, members, now, nil
}

func (endpoint *endpoint) textEntrySets() (*entry.ClosedSets, error) {
	endpoint.textMu.Lock()
	defer endpoint.textMu.Unlock()
	if endpoint.textClosed || endpoint.closedEntryRoot == "" {
		return nil, errors.New("text Entry owner unavailable")
	}
	if endpoint.closedEntries != nil {
		return endpoint.closedEntries, nil
	}
	owner, err := entry.OpenClosedSets(entry.ClosedSetConfig{Root: endpoint.closedEntryRoot, NetworkID: endpoint.network, Current: func() (entry.ClosedSetView, error) {
		_, members, now, err := endpoint.closedTextRoleMembers()
		if err != nil {
			return entry.ClosedSetView{}, err
		}
		view := entry.ClosedSetView{NetworkID: endpoint.network, Now: now}
		for _, member := range members {
			if member.subrole == 1 {
				view.Candidates = append(view.Candidates, member.ClosedSetMember)
			}
		}
		return view, nil
	}})
	if err != nil {
		return nil, err
	}
	endpoint.closedEntries = owner
	return owner, nil
}

func (endpoint *endpoint) closeTextSourceRoots() error {
	endpoint.textMu.Lock()
	defer endpoint.textMu.Unlock()
	return errors.Join(endpoint.closedEntries.Close(), endpoint.closedTokenJournal.Close())
}
