package state

import (
	"crypto/sha256"
	networkdomain "github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"time"
)

// membership retains the authenticated profile next to the domain binding.
// This adapter translates grammar values; Network owns all joining rules.
type membership struct {
	profile      closedprofile.Profile
	participants networkdomain.Membership
	accepted     networkdomain.AcceptedState
}

func (s *networkState) bindCurrentMembership(profile closedprofile.Profile) (membership, error) {
	bound, err := bindMembership(profile, s.current.Candidates)
	if err != nil {
		return membership{}, err
	}
	generation, err := closedProfileGeneration(s.current.Snapshot.Generation)
	if err != nil {
		return membership{}, err
	}
	header := s.current.Header
	facts := networkdomain.EpochFacts{Network: header.NetworkID, Number: header.Number, Digest: header.Digest,
		Previous: header.Previous, ValidFrom: header.ValidFrom, ValidUntil: header.ValidUntil}
	bound.accepted, err = networkdomain.BindAcceptedState(facts, generation, profileFacts(profile), bound.participants)
	return bound, err
}

func bindMembership(profile closedprofile.Profile, candidates []epoch.Candidate) (membership, error) {
	assignments := make([]networkdomain.Assignment, len(profile.Nodes))
	for index, role := range profile.Nodes {
		assignments[index] = networkdomain.Assignment{NodeID: role.NodeID, RecordDigest: role.RecordDigest,
			Generation: role.DutyGeneration, RoleDomain: role.RoleDomain, Subrole: role.Subrole}
	}
	records := make([]networkdomain.NodeRecord, len(candidates))
	for index, record := range candidates {
		records[index] = networkdomain.NodeRecord{NodeID: record.NodeID, RecordDigest: record.RecordDigest, Generation: record.RecordGeneration,
			PublicKey: record.PublicKey, FamilyID: record.FamilyID, KeyID: record.KeyID, DomainProofDigest: sha256.Sum256(record.DomainProof),
			Endpoint: record.Endpoint, CarrierProfile: record.CarrierProfile, Capacity: record.Capacity, Assignment: record.Domain,
			ValidFrom: record.ValidFrom, ValidUntil: record.ValidUntil, AssignmentNotAfter: record.AssignmentNotAfter}
	}
	participants, err := networkdomain.BindMembership(profileBinding(profile), assignments, records)
	if err != nil {
		return membership{}, err
	}
	return membership{profile: profile, participants: participants}, nil
}

func profileBinding(profile closedprofile.Profile) networkdomain.ProfileBinding {
	return networkdomain.ProfileBinding{Network: profile.NetworkID, Generation: profile.StateGeneration, EpochDigest: profile.EpochDigest,
		Digest: profile.Digest, Epoch: profile.Epoch, NotBefore: profile.NotBefore, NotAfter: profile.NotAfter}
}

func profileFacts(profile closedprofile.Profile) networkdomain.ProfileFacts {
	facts := networkdomain.ProfileFacts{ProfileBinding: profileBinding(profile),
		IssuanceAuthorityKey: profile.AuthorityKey, IssuerNodeID: profile.IssuerNodeID,
		TokenKeys: make([]networkdomain.TokenKey, len(profile.Keys))}
	for _, assignment := range profile.Nodes {
		if assignment.NodeID == profile.IssuerNodeID {
			facts.IssuerDutyGeneration = assignment.DutyGeneration
			break
		}
	}
	for index, key := range profile.Keys {
		facts.TokenKeys[index].WindowStart = time.Unix(int64(key.WindowStart), 0).UTC()
		facts.TokenKeys[index].Class = key.Class
		copy(facts.TokenKeys[index].SPKI[:], key.SPKI)
	}
	return facts
}
