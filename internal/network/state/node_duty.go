package state

import "time"

// NodeDuty is the one State-created immutable duty value for a single Node
// lifecycle. It groups the authenticated current Epoch identity and freshness,
// the local signed record with its assignment, and the bounded candidate list.
// It carries no Source attempts, pending Epoch facts, control roots, retained
// issuer-profile bytes, or storage handle: State keeps freshness and conflict
// classification ownership, and Node receives copied facts only (ADR-0104).
type NodeDuty struct {
	// Epoch identity and freshness.
	Generation     string
	NetworkID      [32]byte
	Epoch          uint64
	Digest         [32]byte
	EpochValidFrom time.Time
	ValidUntil     time.Time
	Profile        string
	Fresh          bool
	Conflicting    bool

	// Local signed record and assignment.
	RecordPresent    bool
	NodeID           [32]byte
	NodePublicKey    [32]byte
	RecordGeneration uint64
	RecordValidFrom  time.Time
	RecordValidUntil time.Time
	DeclaredFamily   string
	ProbeEndpoint    string
	CarrierProfile   string
	ProbeCapacity    uint16
	Assignment       string
	AssignmentDigest [32]byte

	// Bounded State-authorized peers.
	CandidateCount uint8
	Candidates     [64]NodeDutyCandidate
}

// NodeDutyCandidate is one narrow State-authorized peer fact inside a
// NodeDuty. It deliberately contains no source, address history, target, or
// complete route material.
type NodeDutyCandidate struct {
	NodeID, PublicKey, KeyID, FamilyID, RecordDigest, DomainProofDigest [32]byte
	Endpoint, CarrierProfile, Assignment                                string
	Capacity                                                            uint16
	ValidFrom, ValidUntil, AssignmentNotAfter                           time.Time
}

// ProjectNodeDuty copies one authenticated Snapshot's duty facts into the
// narrow NodeDuty value. The projection is pure: State's current-generation
// check already owns freshness and conflict classification, and the receiver
// revalidates the candidate bound on receipt.
func ProjectNodeDuty(snapshot Snapshot) NodeDuty {
	duty := NodeDuty{
		Generation:       snapshot.Generation,
		NetworkID:        snapshot.NetworkID,
		Epoch:            snapshot.Epoch,
		Digest:           snapshot.Digest,
		EpochValidFrom:   snapshot.EpochValidFrom,
		ValidUntil:       snapshot.ValidUntil,
		Profile:          snapshot.Profile,
		Fresh:            snapshot.Freshness == "fresh",
		Conflicting:      snapshot.Conflicting,
		RecordPresent:    snapshot.RecordPresent,
		NodeID:           snapshot.NodeID,
		NodePublicKey:    snapshot.NodePublicKey,
		RecordGeneration: snapshot.RecordGeneration,
		RecordValidFrom:  snapshot.RecordValidFrom,
		RecordValidUntil: snapshot.RecordValidUntil,
		DeclaredFamily:   snapshot.DeclaredFamily,
		ProbeEndpoint:    snapshot.ProbeEndpoint,
		CarrierProfile:   snapshot.CarrierProfile,
		ProbeCapacity:    snapshot.ProbeCapacity,
		Assignment:       snapshot.Assignment,
		AssignmentDigest: snapshot.AssignmentDigest,
		CandidateCount:   snapshot.CandidateCount,
	}
	limit := int(snapshot.CandidateCount)
	if limit > len(snapshot.Candidates) {
		limit = len(snapshot.Candidates)
	}
	for index := 0; index < limit; index++ {
		candidate := snapshot.Candidates[index]
		duty.Candidates[index] = NodeDutyCandidate{
			NodeID:             candidate.NodeID,
			PublicKey:          candidate.PublicKey,
			KeyID:              candidate.KeyID,
			FamilyID:           candidate.FamilyID,
			RecordDigest:       candidate.RecordDigest,
			DomainProofDigest:  candidate.DomainProofDigest,
			Endpoint:           candidate.Endpoint,
			CarrierProfile:     candidate.CarrierProfile,
			Assignment:         candidate.Domain,
			Capacity:           candidate.Capacity,
			ValidFrom:          candidate.ValidFrom,
			ValidUntil:         candidate.ValidUntil,
			AssignmentNotAfter: candidate.AssignmentNotAfter,
		}
	}
	return duty
}

// CurrentNodeDuty returns the current authenticated facts required by one
// Node duty lifecycle as one copied value.
func (s *networkState) CurrentNodeDuty() (NodeDuty, error) {
	snapshot, err := s.Current()
	if err != nil {
		return NodeDuty{}, err
	}
	return ProjectNodeDuty(snapshot), nil
}
