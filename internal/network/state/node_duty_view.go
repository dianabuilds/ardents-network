package state

import "time"

// NodeDutyView is the narrow authenticated current-generation view consumed by
// Node. It preserves State ownership of freshness and record facts without
// exposing the broad Snapshot to Node composition.
type NodeDutyView struct{ snapshot Snapshot }

// CurrentNodeDuty returns the current authenticated facts required by one
// Node duty lifecycle.
func (s *networkState) CurrentNodeDuty() (NodeDutyView, error) {
	snapshot, err := s.Current()
	if err != nil {
		return NodeDutyView{}, err
	}
	return NodeDutyView{snapshot: snapshot}, nil
}

func (view NodeDutyView) DutyGeneration() string          { return view.snapshot.Generation }
func (view NodeDutyView) DutyNetworkID() [32]byte         { return view.snapshot.NetworkID }
func (view NodeDutyView) DutyEpoch() uint64               { return view.snapshot.Epoch }
func (view NodeDutyView) DutyDigest() [32]byte            { return view.snapshot.Digest }
func (view NodeDutyView) DutyEpochValidFrom() time.Time   { return view.snapshot.EpochValidFrom }
func (view NodeDutyView) DutyValidUntil() time.Time       { return view.snapshot.ValidUntil }
func (view NodeDutyView) DutyProfile() string             { return view.snapshot.Profile }
func (view NodeDutyView) DutyFresh() bool                 { return view.snapshot.Freshness == "fresh" }
func (view NodeDutyView) DutyConflicting() bool           { return view.snapshot.Conflicting }
func (view NodeDutyView) DutyRecordPresent() bool         { return view.snapshot.RecordPresent }
func (view NodeDutyView) DutyNodeID() [32]byte            { return view.snapshot.NodeID }
func (view NodeDutyView) DutyNodePublicKey() [32]byte     { return view.snapshot.NodePublicKey }
func (view NodeDutyView) DutyRecordValidFrom() time.Time  { return view.snapshot.RecordValidFrom }
func (view NodeDutyView) DutyRecordValidUntil() time.Time { return view.snapshot.RecordValidUntil }
func (view NodeDutyView) DutyDeclaredFamily() string      { return view.snapshot.DeclaredFamily }
func (view NodeDutyView) DutyProbeEndpoint() string       { return view.snapshot.ProbeEndpoint }
func (view NodeDutyView) DutyProbeCapacity() uint16       { return view.snapshot.ProbeCapacity }
func (view NodeDutyView) DutyAssignment() string          { return view.snapshot.Assignment }
func (view NodeDutyView) DutyAssignmentDigest() [32]byte  { return view.snapshot.AssignmentDigest }
func (view NodeDutyView) DutyCandidateCount() uint8       { return view.snapshot.CandidateCount }
func (view NodeDutyView) DutyCandidateNodeID(index uint8) [32]byte {
	if index >= view.snapshot.CandidateCount {
		return [32]byte{}
	}
	return view.snapshot.Candidates[index].NodeID
}
func (view NodeDutyView) DutyCandidatePublicKey(index uint8) [32]byte {
	if index >= view.snapshot.CandidateCount {
		return [32]byte{}
	}
	return view.snapshot.Candidates[index].PublicKey
}
func (view NodeDutyView) DutyCandidateKeyID(index uint8) [32]byte {
	if index >= view.snapshot.CandidateCount {
		return [32]byte{}
	}
	return view.snapshot.Candidates[index].KeyID
}
func (view NodeDutyView) DutyCandidateFamilyID(index uint8) [32]byte {
	if index >= view.snapshot.CandidateCount {
		return [32]byte{}
	}
	return view.snapshot.Candidates[index].FamilyID
}
func (view NodeDutyView) DutyCandidateRecordDigest(index uint8) [32]byte {
	if index >= view.snapshot.CandidateCount {
		return [32]byte{}
	}
	return view.snapshot.Candidates[index].RecordDigest
}
func (view NodeDutyView) DutyCandidateDomainProofDigest(index uint8) [32]byte {
	if index >= view.snapshot.CandidateCount {
		return [32]byte{}
	}
	return view.snapshot.Candidates[index].DomainProofDigest
}
func (view NodeDutyView) DutyCandidateEndpoint(index uint8) string {
	if index >= view.snapshot.CandidateCount {
		return ""
	}
	return view.snapshot.Candidates[index].Endpoint
}
func (view NodeDutyView) DutyCandidateCapacity(index uint8) uint16 {
	if index >= view.snapshot.CandidateCount {
		return 0
	}
	return view.snapshot.Candidates[index].Capacity
}
func (view NodeDutyView) DutyCandidateAssignment(index uint8) string {
	if index >= view.snapshot.CandidateCount {
		return ""
	}
	return view.snapshot.Candidates[index].Domain
}
func (view NodeDutyView) DutyCandidateValidFrom(index uint8) time.Time {
	if index >= view.snapshot.CandidateCount {
		return time.Time{}
	}
	return view.snapshot.Candidates[index].ValidFrom
}
func (view NodeDutyView) DutyCandidateValidUntil(index uint8) time.Time {
	if index >= view.snapshot.CandidateCount {
		return time.Time{}
	}
	return view.snapshot.Candidates[index].ValidUntil
}
func (view NodeDutyView) DutyCandidateAssignmentNotAfter(index uint8) time.Time {
	if index >= view.snapshot.CandidateCount {
		return time.Time{}
	}
	return view.snapshot.Candidates[index].AssignmentNotAfter
}

// DutyAuthorityCount, DutyAuthorityID, and DutyAuthorityPublicKey expose the
// finite current State authority verification set required by a Node-only
// offline Transit Grant check. They do not expose State source or persistence.
func (view NodeDutyView) DutyAuthorityCount() uint8 { return view.snapshot.EpochAuthorityCount }
func (view NodeDutyView) DutyAuthorityID(index uint8) [32]byte {
	if index >= view.snapshot.EpochAuthorityCount {
		return [32]byte{}
	}
	return view.snapshot.EpochAuthorityIDs[index]
}
func (view NodeDutyView) DutyAuthorityPublicKey(index uint8) [32]byte {
	if index >= view.snapshot.EpochAuthorityCount {
		return [32]byte{}
	}
	return view.snapshot.EpochAuthorityKeys[index]
}
