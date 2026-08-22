package state

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
)

var errSourceRoleCollision = errors.New("source role collides with a Candidate View member")

func (s *networkState) rejectSourceCollisions() error {
	for _, decision := range []*candidateDecision{s.currentDecision, s.pendingDecision} {
		if decision != nil && sourceCollides(s.config.sourceInfo, *decision) {
			return fmt.Errorf("%w: identity, family, or endpoint", errSourceRoleCollision)
		}
	}
	return nil
}

func (s *networkState) rejectDecisionSourceCollisions(decision candidateDecision) error {
	if sourceCollides(s.config.sourceInfo, decision) {
		return fmt.Errorf("%w: identity, family, or endpoint", errSourceRoleCollision)
	}
	return nil
}

func sourceCollides(info source.Details, decision candidateDecision) bool {
	if !info.Configured {
		return false
	}
	for index := range info.Identities {
		if epochDecisionCollides(decision.verified, info.Identities[index], info.Families[index], info.EndpointHandles[index]) {
			return true
		}
	}
	return false
}

func epochDecisionCollides(decision verifiedEpochDecision, identity [32]byte, family, endpoint string) bool {
	for _, candidate := range decision.NodeIDs {
		if candidate == identity {
			return true
		}
	}
	for _, candidate := range decision.KeyIDs {
		if candidate == identity {
			return true
		}
	}
	for _, candidate := range decision.Families {
		if candidate == family {
			return true
		}
	}
	for _, candidate := range decision.Endpoints {
		if candidate == endpoint {
			return true
		}
	}
	return false
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

func routeCandidates(decision *candidateDecision) ([64]routeCandidate, uint8) {
	var result [64]routeCandidate
	if decision == nil {
		return result, 0
	}
	verified := decision.verified
	for index := range verified.NodeIDs {
		result[index] = routeCandidate{NodeID: verified.NodeIDs[index], PublicKey: verified.PublicKeys[index], KeyID: verified.KeyIDs[index],
			FamilyID: verified.FamilyIDs[index], RecordDigest: verified.RecordDigests[index], DomainProofDigest: sha256.Sum256(verified.DomainProofs[index]),
			Family: verified.Families[index], Endpoint: verified.Endpoints[index], Capacity: verified.Capacities[index],
			Domain: verified.Domains[index], ValidFrom: verified.ValidFrom[index], ValidUntil: verified.ValidUntil[index],
			AssignmentNotAfter: verified.AssignmentNotAfter[index]}
	}
	return result, uint8(len(verified.NodeIDs))
}
