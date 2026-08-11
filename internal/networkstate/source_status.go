package networkstate

import (
	"errors"
	"fmt"
	"time"
)

var errSourceRoleCollision = errors.New("source role collides with a Candidate View member")

func (s *store) rejectSourceCollisions() error {
	for _, source := range s.config.sources {
		if _, exists := s.config.authorities[source.identity]; exists {
			return errors.New("source identity collides with an Epoch authority")
		}
		for _, decision := range []*candidateDecision{s.currentDecision, s.pendingDecision} {
			if decision != nil {
				if err := sourceCollidesWithDecision(source, *decision); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *store) rejectDecisionSourceCollisions(decision candidateDecision) error {
	for _, source := range s.config.sources {
		if err := sourceCollidesWithDecision(source, decision); err != nil {
			return err
		}
	}
	return nil
}

func sourceCollidesWithDecision(source sourceConfig, decision candidateDecision) error {
	for _, record := range decision.accepted {
		if source.identity == record.nodeID || source.identity == record.keyID ||
			source.family == record.family || source.handle == record.endpoint {
			return fmt.Errorf("%w: identity, family, or endpoint", errSourceRoleCollision)
		}
	}
	return nil
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
