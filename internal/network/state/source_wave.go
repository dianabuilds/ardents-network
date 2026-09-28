package state

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
)

const sourceWaveDuration = 15 * time.Second

func (s *networkState) startSourceWave(now time.Time) ([2]int, time.Time, error) {
	state := s.distribution
	if state.cycleActive {
		if now.Unix() >= state.cycleDeadline {
			for index := range state.attempts {
				state.interruptUnresolvedAttempt(index)
			}
			state.cycleActive = false
			state.sequence++
			if err := applyFailureBackoff(&state, now, state.cycleSeed); err != nil {
				return [2]int{}, time.Time{}, err
			}
			if err := s.commitDistribution(state); err != nil {
				return [2]int{}, time.Time{}, err
			}
			return [2]int{}, time.Time{}, fmt.Errorf("%w: durable source cycle reached its recorded deadline", errRefreshUnavailable)
		}
		changed := false
		for index := sourceDigestSlotOffset; index < len(state.attempts); index++ {
			if state.interruptUnresolvedAttempt(index) {
				changed = true
			}
		}
		if changed {
			state.sequence++
			if err := s.commitDistribution(state); err != nil {
				return [2]int{}, time.Time{}, err
			}
		}
		deadline := time.Unix(state.cycleDeadline, 0)
		if err := s.retainSourceExposures(deadline); err != nil {
			return [2]int{}, time.Time{}, err
		}
		return [2]int{int(state.sourceOrder[0]), int(state.sourceOrder[1])}, deadline, nil
	}
	state.sequence++
	state.trustedTimeFloor = max(state.trustedTimeFloor, now.Unix())
	state.cycleID++
	state.cycleActive = true
	state.cyclePurpose = sourceCyclePurposeRefresh
	state.cycleStarted = now.Unix()
	state.cycleDeadline = now.Add(sourceWaveDuration).Unix()
	state.attempts = [4]byte{}
	state.outcomes = [4]byte{}
	state.requestedDigests = [2][32]byte{}
	state.observedEpochs = [4]uint64{}
	state.observedDigests = [4][32]byte{}
	seed := s.config.sourceInfo.OrderSeed
	if seed == [32]byte{} {
		if _, err := rand.Read(seed[:]); err != nil {
			return [2]int{}, time.Time{}, fmt.Errorf("draw source order: %w", err)
		}
	}
	order := [2]int{0, 1}
	if seed[0]&1 == 1 {
		order = [2]int{1, 0}
	}
	state.cycleSeed = seed
	state.sourceOrder = [2]byte{byte(order[0]), byte(order[1])}
	if err := s.commitDistribution(state); err != nil {
		return order, time.Time{}, err
	}
	deadline := time.Unix(state.cycleDeadline, 0)
	if err := s.retainSourceExposures(deadline); err != nil {
		return order, time.Time{}, err
	}
	return order, deadline, nil
}

func (s *networkState) completeSourceWave(started time.Time, base *epoch.Decision, results []sourceResult) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() { s.refreshing = false }()
	if s.closed {
		return Snapshot{}, errors.New("network state is closed")
	}
	if !sameGeneration(s.current, base) {
		return Snapshot{}, errors.New("network state changed during the finite source wave")
	}
	summary := summarizeSourceWave(results)
	if summary.collisionErr != nil {
		if err := s.recordSourceConflict(started, summary.outcomes, summary.observedEpochs, summary.observedDigests); err != nil {
			return Snapshot{}, err
		}
		return Snapshot{}, summary.collisionErr
	}
	if sourceConflict(summary.valid) {
		if err := s.recordSourceConflict(started, summary.outcomes, summary.observedEpochs, summary.observedDigests); err != nil {
			return Snapshot{}, err
		}
		return Snapshot{}, errors.New("sources exposed threshold-valid conflicting Epochs")
	}
	if len(summary.valid) > 0 {
		selected := newestSourceDecision(summary.valid)
		if err := s.allowCandidateTransition(selected); err != nil {
			if err := s.recordSourceConflict(started, summary.outcomes, summary.observedEpochs, summary.observedDigests); err != nil {
				return Snapshot{}, err
			}
			return Snapshot{}, err
		}
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
	selected := newestSourceDecision(summary.valid)
	if now.Before(selected.Header.ValidFrom) {
		return s.commitPendingSourceWave(now, selected, summary)
	}
	if !now.Before(selected.Header.ValidUntil) {
		if err := s.commitSourceFailure(now, summary.outcomes, summary.observedEpochs, summary.observedDigests); err != nil {
			return Snapshot{}, err
		}
		return Snapshot{}, errors.Join(errRefreshUnavailable, errors.New("selected Epoch expired before source wave completed"))
	}
	return s.commitActiveSourceWave(now, selected, summary)
}

func (s *networkState) recordSourceConflict(now time.Time, outcomes [4]byte, epochs [4]uint64, digests [4][32]byte) error {
	state := s.distribution
	state.observedEpochs, state.observedDigests = epochs, digests
	if err := finishWaveState(&state, now, outcomes); err != nil {
		return err
	}
	state.conflicting = true
	return s.commitDistribution(state)
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

func sourceConflict(valid []epoch.Decision) bool {
	for first := range valid {
		for second := first + 1; second < len(valid); second++ {
			if valid[first].Header.Number == valid[second].Header.Number && valid[first].Header.Digest != valid[second].Header.Digest {
				return true
			}
		}
	}
	return false
}

type sourceWaveSummary struct {
	valid           []epoch.Decision
	outcomes        [4]byte
	observedEpochs  [4]uint64
	observedDigests [4][32]byte
	collisionErr    error
}

func summarizeSourceWave(results []sourceResult) sourceWaveSummary {
	summary := sourceWaveSummary{valid: make([]epoch.Decision, 0, 2)}
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

func newestSourceDecision(valid []epoch.Decision) epoch.Decision {
	selected := valid[0]
	for _, candidate := range valid[1:] {
		if candidate.Header.Number > selected.Header.Number {
			selected = candidate
		}
	}
	return selected
}

func (s *networkState) commitPendingSourceWave(now time.Time, selected epoch.Decision, summary sourceWaveSummary) (Snapshot, error) {
	if s.current == nil {
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
	if err := s.retainSourceExposures(selected.Header.ValidUntil); err != nil {
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
	if err := s.retainSourceExposures(selected.Header.ValidUntil); err != nil {
		return Snapshot{}, err
	}
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
