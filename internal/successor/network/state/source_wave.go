package state

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
)

func (s *networkState) startSourceWave(now time.Time) ([2]int, time.Time, error) {
	seed := s.config.sourceInfo.OrderSeed
	if !s.distribution.cycleActive && seed == [32]byte{} {
		if _, err := rand.Read(seed[:]); err != nil {
			return [2]int{}, time.Time{}, fmt.Errorf("draw source order: %w", err)
		}
	}
	state, expired, err := proposeSourceCycle(s.distribution, now, seed)
	if err != nil {
		return [2]int{}, time.Time{}, err
	}
	order := [2]int{int(state.sourceOrder[0]), int(state.sourceOrder[1])}
	if state.sequence != s.distribution.sequence {
		if err := s.commitDistribution(state); err != nil {
			return order, time.Time{}, err
		}
	}
	if expired {
		return [2]int{}, time.Time{}, fmt.Errorf("%w: durable source cycle reached its recorded deadline", errRefreshUnavailable)
	}
	deadline := time.Unix(state.cycleDeadline, 0)
	if err := s.holdSourceExposures(deadline); err != nil {
		return order, time.Time{}, err
	}
	return order, deadline, nil
}
func (s *networkState) completeSourceWave(started time.Time, base *epoch.Decision, results []sourceResult) (Snapshot, error) {
	return s.completeSourceWaveWithConflictCommit(started, base, results, s.storage.CommitControl)
}

func (s *networkState) completeSourceWaveWithConflictCommit(started time.Time, base *epoch.Decision, results []sourceResult, commit func(string, []byte) error) (snapshot Snapshot, resultErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() {
		// Keep refresh ownership until the live contact guard has been demoted.
		// Failed or uncertain terminal writes leave the guard live.
		if !s.closed && !s.distribution.cycleActive {
			resultErr = errors.Join(resultErr, s.releaseSourceWaveLocked())
		}
		s.refreshing = false
	}()
	if s.closed {
		return Snapshot{}, errors.New("network state is closed")
	}
	if !sameGeneration(s.current, base) {
		return Snapshot{}, errors.New("network state changed during the finite source wave")
	}
	summary := summarizeSourceWave(s.distribution, results)
	if summary.collisionErr != nil {
		if err := s.recordSourceConflictWithControl(started, summary.outcomes, summary.observedEpochs, summary.observedDigests, commit); err != nil {
			return Snapshot{}, err
		}
		return Snapshot{}, summary.collisionErr
	}
	selected, selection, selectionErr := s.selectEpochHistory(summary.valid)
	if len(summary.valid) > 0 && selectionErr != nil {
		if isEpochHistoryConflict(selectionErr) {
			if err := s.recordSourceConflictWithControl(started, summary.outcomes, summary.observedEpochs, summary.observedDigests, commit); err != nil {
				return Snapshot{}, err
			}
		}
		return Snapshot{}, selectionErr
	}
	now, err := trustedNow(s.config, s.distribution)
	if err != nil {
		return Snapshot{}, err
	}
	if len(summary.valid) == 0 {
		if err := s.commitSourceFailure(now, summary.outcomes, summary.observedEpochs, summary.observedDigests); err != nil {
			return Snapshot{}, err
		}
		failures := []error{errRefreshUnavailable, errors.New("finite source wave produced no valid state")}
		for _, result := range results {
			if result.err != nil {
				failures = append(failures, fmt.Errorf("source %d: %w", result.index+1, result.err))
			}
		}
		return Snapshot{}, errors.Join(failures...)
	}
	return s.publishSelectedEpoch(selection, now, selected, summary)
}

func (s *networkState) recordSourceConflictWithControl(now time.Time, outcomes [4]byte, epochs [4]uint64, digests [4][32]byte, commit func(string, []byte) error) error {
	state := s.distribution
	state.observedEpochs, state.observedDigests = epochs, digests
	if err := finishWaveState(&state, now, outcomes); err != nil {
		return err
	}
	state.conflicting = true
	return s.commitDistributionWithControl(state, commit)
}

func sameGeneration(current, base *epoch.Decision) bool {
	if current == nil || base == nil {
		return current == nil && base == nil
	}
	return current.Snapshot.Generation == base.Snapshot.Generation && current.Snapshot.Digest == base.Snapshot.Digest
}

func (s *networkState) commitSourceFailure(now time.Time, outcomes [4]byte, epochs [4]uint64, digests [4][32]byte) error {
	state := s.distribution
	state.observedEpochs, state.observedDigests = epochs, digests
	if err := finishWaveState(&state, now, outcomes); err != nil {
		return err
	}
	return s.commitDistribution(state)
}

type sourceWaveSummary struct {
	valid           []epoch.Decision
	outcomes        [4]byte
	observedEpochs  [4]uint64
	observedDigests [4][32]byte
	collisionErr    error
}

func summarizeSourceWave(state distributionState, results []sourceResult) sourceWaveSummary {
	// Start from the same durable cycle: a consumed selector has no new result
	// on resume, but its recorded outcome and evidence still belong to this wave.
	summary := sourceWaveSummary{valid: make([]epoch.Decision, 0, 2), outcomes: state.outcomes,
		observedEpochs: state.observedEpochs, observedDigests: state.observedDigests}
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
			summary.observedEpochs[result.slot] = result.decision.Header.Number
			summary.observedDigests[result.slot] = result.decision.Header.Digest
		}
	}
	return summary
}

func (s *networkState) commitDeferredSourceWave(now time.Time, selected epoch.Decision, summary sourceWaveSummary) (Snapshot, error) {
	// A pending generation requires an active predecessor on reopen. Keep
	// authenticated Source observations, but do not stage a future genesis.
	state := s.distribution
	state.observedEpochs, state.observedDigests = summary.observedEpochs, summary.observedDigests
	if err := finishWaveState(&state, now, summary.outcomes); err != nil {
		return Snapshot{}, err
	}
	state.nextAutomatic = max(state.nextAutomatic, selected.Header.ValidFrom.Unix())
	if err := s.commitDistribution(state); err != nil {
		return Snapshot{}, err
	}
	return Snapshot{}, errors.Join(errRefreshUnavailable, errors.New("genesis Epoch is not yet current"))

}

func (s *networkState) commitPendingSourceWave(now time.Time, selected epoch.Decision, summary sourceWaveSummary) (Snapshot, error) {
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
	state.pendingDigest, state.pendingValidFrom = selected.Header.Digest, selected.Header.ValidFrom.Unix()
	if err := s.commitDistribution(state); err != nil {
		return Snapshot{}, err
	}
	if newPending {
		s.pendingDecision = &selected
	}
	return s.snapshotWithDistribution(now), nil
}

func (s *networkState) commitActiveSourceWave(now time.Time, selected epoch.Decision, summary sourceWaveSummary) (Snapshot, error) {
	state := s.distribution
	state.observedEpochs, state.observedDigests = summary.observedEpochs, summary.observedDigests
	if err := finishWaveState(&state, now, summary.outcomes); err != nil {
		return Snapshot{}, err
	}
	state.epochFloor, state.epochDigest = selected.Header.Number, selected.Header.Digest
	state.trustedTimeFloor = max(state.trustedTimeFloor, now.Unix())
	if state.pendingDigest == selected.Header.Digest {
		state.pendingDigest, state.pendingValidFrom = [32]byte{}, 0
	}
	if s.current == nil || selected.Header.Digest != s.current.Snapshot.Digest {
		if err := s.commitActiveDecision(selected, state); err != nil {
			return Snapshot{}, err
		}
	} else if err := s.commitDistribution(state); err != nil {
		return Snapshot{}, err
	}
	if s.pendingDecision != nil && s.pendingDecision.Header.Digest == selected.Header.Digest {
		s.pendingDecision = nil
	}
	return s.snapshotWithDistribution(now), nil
}
