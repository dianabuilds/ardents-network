package node

import "time"

// Test snapshots implement the external DutyView seam; production projects
// authenticated State through state.NodeDutyView and copies it with
// currentFacts (ADR-0101). The projection is complete: every getter returns
// one field of the Node-owned immutable snapshot, and the candidate and
// authority getters refuse indexes beyond the copied counts.

func (facts dutyFacts) DutyGeneration() string          { return facts.Generation }
func (facts dutyFacts) DutyNetworkID() [32]byte         { return facts.NetworkID }
func (facts dutyFacts) DutyEpoch() uint64               { return facts.Epoch }
func (facts dutyFacts) DutyDigest() [32]byte            { return facts.Digest }
func (facts dutyFacts) DutyEpochValidFrom() time.Time   { return facts.EpochValidFrom }
func (facts dutyFacts) DutyValidUntil() time.Time       { return facts.ValidUntil }
func (facts dutyFacts) DutyProfile() string             { return facts.Profile }
func (facts dutyFacts) DutyFresh() bool                 { return facts.Fresh }
func (facts dutyFacts) DutyConflicting() bool           { return facts.Conflicting }
func (facts dutyFacts) DutyRecordPresent() bool         { return facts.RecordPresent }
func (facts dutyFacts) DutyNodeID() [32]byte            { return facts.NodeID }
func (facts dutyFacts) DutyNodePublicKey() [32]byte     { return facts.NodePublicKey }
func (facts dutyFacts) DutyRecordGeneration() uint64    { return facts.RecordGeneration }
func (facts dutyFacts) DutyRecordValidFrom() time.Time  { return facts.RecordValidFrom }
func (facts dutyFacts) DutyRecordValidUntil() time.Time { return facts.RecordValidUntil }
func (facts dutyFacts) DutyDeclaredFamily() string      { return facts.DeclaredFamily }
func (facts dutyFacts) DutyProbeEndpoint() string       { return facts.ProbeEndpoint }
func (facts dutyFacts) DutyCarrierProfile() string      { return facts.CarrierProfile }
func (facts dutyFacts) DutyProbeCapacity() uint16       { return facts.ProbeCapacity }
func (facts dutyFacts) DutyAssignment() string          { return facts.Assignment }
func (facts dutyFacts) DutyAssignmentDigest() [32]byte  { return facts.AssignmentDigest }
func (facts dutyFacts) DutyCandidateCount() uint8       { return facts.CandidateCount }
func (facts dutyFacts) DutyCandidateNodeID(index uint8) [32]byte {
	if index >= facts.CandidateCount {
		return [32]byte{}
	}
	return facts.Candidates[index].NodeID
}
func (facts dutyFacts) DutyCandidatePublicKey(index uint8) [32]byte {
	if index >= facts.CandidateCount {
		return [32]byte{}
	}
	return facts.Candidates[index].PublicKey
}
func (facts dutyFacts) DutyCandidateKeyID(index uint8) [32]byte {
	if index >= facts.CandidateCount {
		return [32]byte{}
	}
	return facts.Candidates[index].KeyID
}
func (facts dutyFacts) DutyCandidateFamilyID(index uint8) [32]byte {
	if index >= facts.CandidateCount {
		return [32]byte{}
	}
	return facts.Candidates[index].FamilyID
}
func (facts dutyFacts) DutyCandidateRecordDigest(index uint8) [32]byte {
	if index >= facts.CandidateCount {
		return [32]byte{}
	}
	return facts.Candidates[index].RecordDigest
}
func (facts dutyFacts) DutyCandidateDomainProofDigest(index uint8) [32]byte {
	if index >= facts.CandidateCount {
		return [32]byte{}
	}
	return facts.Candidates[index].DomainProofDigest
}
func (facts dutyFacts) DutyCandidateEndpoint(index uint8) string {
	if index >= facts.CandidateCount {
		return ""
	}
	return facts.Candidates[index].Endpoint
}
func (facts dutyFacts) DutyCandidateCarrierProfile(index uint8) string {
	if index >= facts.CandidateCount {
		return ""
	}
	return facts.Candidates[index].CarrierProfile
}
func (facts dutyFacts) DutyCandidateCapacity(index uint8) uint16 {
	if index >= facts.CandidateCount {
		return 0
	}
	return facts.Candidates[index].Capacity
}
func (facts dutyFacts) DutyCandidateAssignment(index uint8) string {
	if index >= facts.CandidateCount {
		return ""
	}
	return facts.Candidates[index].Assignment
}
func (facts dutyFacts) DutyCandidateValidFrom(index uint8) time.Time {
	if index >= facts.CandidateCount {
		return time.Time{}
	}
	return facts.Candidates[index].ValidFrom
}
func (facts dutyFacts) DutyCandidateValidUntil(index uint8) time.Time {
	if index >= facts.CandidateCount {
		return time.Time{}
	}
	return facts.Candidates[index].ValidUntil
}
func (facts dutyFacts) DutyCandidateAssignmentNotAfter(index uint8) time.Time {
	if index >= facts.CandidateCount {
		return time.Time{}
	}
	return facts.Candidates[index].AssignmentNotAfter
}
func (facts dutyFacts) DutyAuthorityCount() uint8 { return facts.AuthorityCount }
func (facts dutyFacts) DutyAuthorityID(index uint8) [32]byte {
	if index >= facts.AuthorityCount {
		return [32]byte{}
	}
	return facts.Authorities[index].ID
}
func (facts dutyFacts) DutyAuthorityPublicKey(index uint8) [32]byte {
	if index >= facts.AuthorityCount {
		return [32]byte{}
	}
	return facts.Authorities[index].PublicKey
}
