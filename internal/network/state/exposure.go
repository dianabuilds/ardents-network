package state

import (
	"errors"
	"fmt"
	"time"

	stateepoch "github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/source"
)

var errSourceRoleCollision = errors.New("source role collides with a Candidate View member")

func (s *store) rejectSourceCollisions() error {
	for _, decision := range []*candidateDecision{s.currentDecision, s.pendingDecision} {
		if decision != nil && sourceCollides(s.config.sourceInfo, *decision) {
			return fmt.Errorf("%w: identity, family, or endpoint", errSourceRoleCollision)
		}
	}
	return nil
}

func (s *store) rejectDecisionSourceCollisions(decision candidateDecision) error {
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

func epochDecisionCollides(decision stateepoch.Decision, identity [32]byte, family, endpoint string) bool {
	for _, candidate := range decision.Identities {
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

func (s *store) snapshotWithDistribution(now time.Time) Snapshot {
	if s.current == nil {
		return Snapshot{}
	}
	snapshot := *s.current
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
