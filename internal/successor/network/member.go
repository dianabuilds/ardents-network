package network

import "time"

// Member is copied public membership data. Mutation grants no authority and
// cannot alter the bound Membership from which the copy was obtained.
type Member struct {
	NodeID, RecordDigest, PublicKey, FamilyID, KeyID, DomainProofDigest [32]byte
	DutyGeneration                                                      uint64
	RoleDomain, Subrole                                                 uint8
	Endpoint, CarrierProfile, Assignment                                string
	Capacity                                                            uint16
	ValidFrom, ValidUntil, AssignmentNotAfter                           time.Time
	observed                                                            time.Time
}

func (member Member) Current(now time.Time) bool {
	if now.IsZero() || member.observed.IsZero() {
		return false
	}
	if now.Before(member.observed) {
		now = member.observed
	}
	return member.NodeID != [32]byte{} && member.PublicKey != [32]byte{} && member.FamilyID != [32]byte{} && member.RecordDigest != [32]byte{} &&
		member.DutyGeneration != 0 && member.Capacity != 0 && !now.Before(member.ValidFrom) && now.Before(member.ValidUntil) && now.Before(member.AssignmentNotAfter)
}

func (member Member) NotAfter() time.Time {
	if member.AssignmentNotAfter.Before(member.ValidUntil) {
		return member.AssignmentNotAfter
	}
	return member.ValidUntil
}
