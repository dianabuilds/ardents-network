package state

import (
	"errors"
	"time"
)

type sourceWaveSummary struct {
	valid           []candidateDecision
	outcomes        [4]byte
	observedEpochs  [4]uint64
	observedDigests [4][32]byte
	collisionErr    error
}

func summarizeSourceWave(results []sourceResult) sourceWaveSummary {
	summary := sourceWaveSummary{valid: make([]candidateDecision, 0, 2)}
	for _, result := range results {
		for index, outcome := range result.observations {
			if outcome != 0 {
				summary.outcomes[index] = outcome
			}
		}
		if errors.Is(result.err, errSourceRoleCollision) && summary.collisionErr == nil {
			summary.collisionErr = result.err
		}
		if result.err == nil {
			summary.valid = append(summary.valid, result.decision)
			summary.observedEpochs[result.slot] = result.decision.epoch.number
			summary.observedDigests[result.slot] = result.decision.epoch.digest
		}
	}
	return summary
}

func newestSourceDecision(valid []candidateDecision) candidateDecision {
	selected := valid[0]
	for _, candidate := range valid[1:] {
		if candidate.epoch.number > selected.epoch.number {
			selected = candidate
		}
	}
	return selected
}

func (s *networkState) commitPendingSourceWave(now time.Time, selected candidateDecision, summary sourceWaveSummary) (Snapshot, error) {
	if err := s.retainSourceExposures(selected.epoch.validUntil); err != nil {
		return Snapshot{}, err
	}
	newPending := s.pendingDecision == nil
	if newPending {
		if err := stageGeneration(s.storage, selected); err != nil {
			return Snapshot{}, err
		}
	}
	state := s.distribution
	state.observedEpochs, state.observedDigests = summary.observedEpochs, summary.observedDigests
	if err := finishWaveState(&state, now, summary.outcomes); err != nil {
		return Snapshot{}, err
	}
	state.pendingDigest, state.pendingValidFrom = selected.epoch.digest, selected.epoch.validFrom.Unix()
	if err := s.commitDistribution(state); err != nil {
		return Snapshot{}, err
	}
	if newPending {
		s.pendingDecision = &selected
	}
	return s.snapshotWithDistribution(now), nil
}

func (s *networkState) commitActiveSourceWave(now time.Time, selected candidateDecision, summary sourceWaveSummary) (Snapshot, error) {
	if err := s.retainSourceExposures(selected.epoch.validUntil); err != nil {
		return Snapshot{}, err
	}
	state := s.distribution
	state.observedEpochs, state.observedDigests = summary.observedEpochs, summary.observedDigests
	if err := finishWaveState(&state, now, summary.outcomes); err != nil {
		return Snapshot{}, err
	}
	state.epochFloor, state.epochDigest = selected.epoch.number, selected.epoch.digest
	state.trustedTimeFloor = max(state.trustedTimeFloor, now.Unix())
	if state.pendingDigest == selected.epoch.digest {
		state.pendingDigest, state.pendingValidFrom = [32]byte{}, 0
	}
	if s.current == nil || selected.epoch.digest != s.current.Digest {
		if err := s.commitActiveDecision(selected, state); err != nil {
			return Snapshot{}, err
		}
	} else if err := s.commitDistribution(state); err != nil {
		return Snapshot{}, err
	}
	if s.pendingDecision != nil && s.pendingDecision.epoch.digest == selected.epoch.digest {
		s.pendingDecision = nil
	}
	return s.snapshotWithDistribution(now), nil
}
