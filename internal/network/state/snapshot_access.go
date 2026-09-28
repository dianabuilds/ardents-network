package state

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
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
	snapshot := snapshotFromEpoch(s.current.Snapshot)
	snapshot.Candidates, snapshot.CandidateCount = routeCandidates(s.current)
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
		snapshot.PendingEpoch = s.pendingDecision.Header.Number
		snapshot.PendingDigest = s.pendingDecision.Header.Digest
		snapshot.PendingAt = s.pendingDecision.Header.ValidFrom
	}
	switch {
	case snapshot.Conflicting:
		snapshot.Freshness = "conflicting"
	case now.Before(s.current.Header.ValidFrom):
		snapshot.Freshness = "staged"
	case !now.Before(snapshot.ValidUntil):
		snapshot.Freshness = "expired"
	default:
		snapshot.Freshness = "fresh"
	}
	return snapshot
}

func routeCandidates(decision *epoch.Decision) ([64]routeCandidate, uint8) {
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
