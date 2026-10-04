package network

import (
	"errors"
	"time"
)

// Assignment is a role assertion from the authenticated network profile.
// Authentication alone does not establish its correspondence to a Node Record.
type Assignment struct {
	NodeID, RecordDigest [32]byte
	Generation           uint64
	RoleDomain, Subrole  uint8
}

// NodeRecord contains the public facts supplied by the authenticated Epoch.
// Its assignment is established by that Epoch, independently of the profile.
type NodeRecord struct {
	NodeID, RecordDigest, PublicKey, FamilyID, KeyID, DomainProofDigest [32]byte
	Generation                                                          uint64
	Endpoint, CarrierProfile, Assignment                                string
	Capacity                                                            uint16
	ValidFrom, ValidUntil, AssignmentNotAfter                           time.Time
}

// Membership is an immutable correspondence between signed assignments and
// exact records. It retains expired members so filtering cannot hide ambiguity.
type Membership struct {
	profile  ProfileBinding
	members  []Member
	byNode   map[[32]byte]int
	byKey    map[[32]byte]int
	observed time.Time
}

func BindMembership(profile ProfileBinding, assignments []Assignment, records []NodeRecord) (Membership, error) {
	if profile.Network == [32]byte{} || profile.Generation == [32]byte{} || profile.EpochDigest == [32]byte{} || profile.Digest == [32]byte{} || profile.Epoch == 0 || !profile.NotAfter.After(profile.NotBefore) {
		return Membership{}, errors.New("membership profile identity is invalid")
	}
	available := make(map[[32]byte]NodeRecord, len(records))
	for _, record := range records {
		if _, exists := available[record.NodeID]; exists {
			return Membership{}, errors.New("network membership has ambiguous Node Records")
		}
		available[record.NodeID] = record
	}
	bound := Membership{members: make([]Member, 0, len(assignments)), byNode: make(map[[32]byte]int), byKey: make(map[[32]byte]int)}
	bound.profile = profile
	for _, assignment := range assignments {
		record, exists := available[assignment.NodeID]
		_, duplicate := bound.byNode[assignment.NodeID]
		if !exists || duplicate || record.RecordDigest != assignment.RecordDigest || record.Generation != assignment.Generation ||
			(record.CarrierProfile != "ardents-carrier-tcp-tls-v2" && record.CarrierProfile != "ardents-carrier-quic-v2") {
			return Membership{}, errors.New("network assignment has no unique matching Node Record")
		}
		role := roleDomain(record.Assignment)
		if role == 0 || role != assignment.RoleDomain {
			return Membership{}, errors.New("network assignment contradicts the Epoch domain")
		}
		index := len(bound.members)
		bound.byNode[record.NodeID] = index
		if _, exists := bound.byKey[record.PublicKey]; exists {
			bound.byKey[record.PublicKey] = -1
		} else {
			bound.byKey[record.PublicKey] = index
		}
		bound.members = append(bound.members, Member{NodeID: record.NodeID, RecordDigest: record.RecordDigest, DutyGeneration: record.Generation,
			RoleDomain: role, Subrole: assignment.Subrole, PublicKey: record.PublicKey, FamilyID: record.FamilyID, KeyID: record.KeyID,
			DomainProofDigest: record.DomainProofDigest, Endpoint: record.Endpoint, CarrierProfile: record.CarrierProfile, Capacity: record.Capacity,
			ValidFrom: record.ValidFrom, ValidUntil: record.ValidUntil, AssignmentNotAfter: record.AssignmentNotAfter, Assignment: record.Assignment})
	}
	return bound, nil
}

// Observe binds member use to the owner's trusted observation. Older caller
// timestamps cannot extend a member's validity. It does not accept a profile.
func (membership Membership) Observe(now time.Time) Membership {
	membership.members = append([]Member(nil), membership.members...)
	membership.observed = now
	for index := range membership.members {
		membership.members[index].observed = now
	}
	return membership
}

func (membership Membership) Members() []Member { return append([]Member(nil), membership.members...) }

func (membership Membership) Member(id [32]byte, now time.Time) (Member, error) {
	index, exists := membership.byNode[id]
	return membership.lookup(id, index, exists, now)
}

func (membership Membership) MemberByKey(key [32]byte, now time.Time) (Member, error) {
	index, exists := membership.byKey[key]
	return membership.lookup(key, index, exists, now)
}

func (membership Membership) lookup(identity [32]byte, index int, exists bool, now time.Time) (Member, error) {
	if membership.observed.IsZero() || identity == [32]byte{} || !exists || index < 0 || !membership.members[index].Current(now) {
		return Member{}, errors.New("network member is absent, ambiguous or no longer current")
	}
	return membership.members[index], nil
}

// roleDomain is the fixed closed-profile correspondence. Retired assignments
// have no role in this membership even when their historical grammar verifies.
func roleDomain(assignment string) uint8 {
	switch assignment {
	case "initiator":
		return 1
	case "rendezvous":
		return 2
	case "responder":
		return 3
	case "introduction":
		return 4
	default:
		return 0
	}
}
