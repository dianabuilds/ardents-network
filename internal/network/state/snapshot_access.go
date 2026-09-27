package state

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"time"
)

// ErrNoCurrentGeneration identifies an owned State root which has not yet
// accepted its first authenticated generation. Callers with a configured
// finite Source plan may use this narrow condition to bootstrap synchronously.
var ErrNoCurrentGeneration = errors.New("network state has no current generation")

// Current returns a copy of the current immutable Snapshot.
func (s *networkState) Current() (Snapshot, error) {
	if s.resourceGuard != nil {
		if err := s.resourceGuard.Check(); err != nil {
			return Snapshot{}, err
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return Snapshot{}, errors.New("network state is closed")
	}
	if s.automaticErr != nil {
		return Snapshot{}, fmt.Errorf("automatic Network State refresh failed: %w", s.automaticErr)
	}
	if s.resourceErr != nil {
		return Snapshot{}, fmt.Errorf("H3-S resource governor failed: %w", s.resourceErr)
	}
	if s.current == nil {
		return Snapshot{}, ErrNoCurrentGeneration
	}
	now, err := trustedNow(s.config, s.distribution)
	if err != nil && s.config.sourceInfo.Configured {
		snapshot := s.snapshotWithDistribution(s.config.clock().UTC())
		snapshot.Freshness = "clock-uncertain"
		return snapshot, nil
	}
	if err != nil {
		now = s.config.clock().UTC()
	}
	return s.snapshotWithDistribution(now), nil
}

func (s *networkState) snapshotWithDistribution(now time.Time) Snapshot {
	if s.current == nil {
		return Snapshot{}
	}
	snapshot := *s.current
	ids := make([][32]byte, 0, len(s.config.authorities))
	for id := range s.config.authorities {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })
	snapshot.EpochAuthorityCount, snapshot.EpochThreshold = uint8(len(ids)), uint8(s.config.threshold)
	for index, id := range ids {
		snapshot.EpochAuthorityIDs[index] = id
		copy(snapshot.EpochAuthorityKeys[index][:], s.config.authorities[id])
	}
	snapshot.Candidates, snapshot.CandidateCount = routeCandidates(s.currentDecision)
	snapshot.Conflicting = s.distribution.conflicting
	snapshot.SourceAttempts = uint16(len(s.distribution.history))
	snapshot.LatestCompleteness = "latest completeness unproven"
	snapshot.ObservedEpochs = s.distribution.observedEpochs
	snapshot.ObservedDigests = s.distribution.observedDigests
	for index, outcome := range s.distribution.outcomes {
		snapshot.SourceOutcomes[index] = sourceOutcomeName(outcome)
	}
	if s.distribution.trustedTimeFloor != 0 {
		snapshot.TrustedTime = time.Unix(s.distribution.trustedTimeFloor, 0).UTC()
	}
	if s.distribution.nextAutomatic != 0 {
		snapshot.NextAutomatic = time.Unix(s.distribution.nextAutomatic, 0).UTC()
	}
	if s.pendingDecision != nil {
		snapshot.PendingEpoch = s.pendingDecision.epoch.number
		snapshot.PendingDigest = s.pendingDecision.epoch.digest
		snapshot.PendingAt = s.pendingDecision.epoch.validFrom
	}
	switch {
	case snapshot.Conflicting:
		snapshot.Freshness = "conflicting"
	case s.currentDecision != nil && now.Before(s.currentDecision.epoch.validFrom):
		snapshot.Freshness = "staged"
	case !now.Before(snapshot.ValidUntil):
		snapshot.Freshness = "expired"
	default:
		snapshot.Freshness = "fresh"
	}
	return snapshot
}

// BridgeCandidateByKey returns one immutable authenticated Invite-issuer
// projection. Key lookup lets callers authenticate bytes before classifying
// the asserted identity or role.
func (snapshot Snapshot) BridgeCandidateByKey(keyID [32]byte) (BridgeCandidate, bool) {
	for _, candidate := range snapshot.Candidates[:snapshot.CandidateCount] {
		if candidate.KeyID == keyID {
			return BridgeCandidate{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey,
				KeyID: candidate.KeyID, FamilyID: candidate.FamilyID, RecordDigest: candidate.RecordDigest,
				DomainProofDigest: candidate.DomainProofDigest, Domain: candidate.Domain,
				ValidFrom: candidate.ValidFrom, ValidUntil: candidate.ValidUntil,
				AssignmentNotAfter: candidate.AssignmentNotAfter}, true
		}
	}
	return BridgeCandidate{}, false
}

func routeCandidates(decision *verifiedEpochDecision) ([64]routeCandidate, uint8) {
	var result [64]routeCandidate
	if decision == nil {
		return result, 0
	}
	for index, candidate := range decision.Candidates {
		result[index] = routeCandidate{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, KeyID: candidate.KeyID,
			FamilyID: candidate.FamilyID, RecordDigest: candidate.RecordDigest, DomainProofDigest: sha256.Sum256(candidate.DomainProof),
			Family: candidate.Family, Endpoint: candidate.Endpoint, CarrierProfile: candidate.CarrierProfile, Capacity: candidate.Capacity,
			Domain: candidate.Domain, ValidFrom: candidate.ValidFrom, ValidUntil: candidate.ValidUntil,
			AssignmentNotAfter: candidate.AssignmentNotAfter}
	}
	return result, uint8(len(decision.Candidates))
}
