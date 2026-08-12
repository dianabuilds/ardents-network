package state

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"
)

func (s *networkState) startSourceWave(now time.Time) ([2]int, time.Time, error) {
	state := s.distribution
	if state.cycleActive {
		if now.Unix() >= state.cycleDeadline {
			for index, status := range state.attempts {
				if status == 1 {
					state.attempts[index] = 3
					state.outcomes[index] = sourceOutcomeInterrupted
				}
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
		for index := 2; index < len(state.attempts); index++ {
			if state.attempts[index] == 1 {
				state.attempts[index] = 3
				state.outcomes[index] = sourceOutcomeInterrupted
				changed = true
			}
		}
		if changed {
			state.sequence++
			if err := s.commitDistribution(state); err != nil {
				return [2]int{}, time.Time{}, err
			}
		}
		return [2]int{int(state.sourceOrder[0]), int(state.sourceOrder[1])}, time.Unix(state.cycleDeadline, 0), nil
	}
	state.sequence++
	state.trustedTimeFloor = max(state.trustedTimeFloor, now.Unix())
	state.cycleID++
	state.cycleActive = true
	state.cyclePurpose = 1
	state.cycleStarted = now.Unix()
	state.cycleDeadline = now.Add(15 * time.Second).Unix()
	state.attempts = [4]byte{}
	state.outcomes = [4]byte{}
	state.requestedDigests = [2][32]byte{}
	state.observedEpochs = [4]uint64{}
	state.observedDigests = [4][32]byte{}
	seed := s.config.sourceInfo.OrderSeed
	if isZero32(seed) {
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
	return order, time.Unix(state.cycleDeadline, 0), nil
}

func (s *networkState) completeSourceWave(now time.Time, base *Snapshot, results []sourceResult) (Snapshot, error) {
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
		if err := s.recordSourceConflict(now, summary.outcomes, summary.observedEpochs, summary.observedDigests); err != nil {
			return Snapshot{}, err
		}
		return Snapshot{}, summary.collisionErr
	}
	if sourceConflict(summary.valid) {
		if err := s.recordSourceConflict(now, summary.outcomes, summary.observedEpochs, summary.observedDigests); err != nil {
			return Snapshot{}, err
		}
		return Snapshot{}, errors.New("sources exposed threshold-valid conflicting Epochs")
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
	if s.pendingDecision != nil && selected.epoch.number == s.pendingDecision.epoch.number && selected.epoch.digest != s.pendingDecision.epoch.digest {
		if err := s.recordSourceConflict(now, summary.outcomes, summary.observedEpochs, summary.observedDigests); err != nil {
			return Snapshot{}, err
		}
		return Snapshot{}, errors.New("source Epoch conflicts with the durable pending Epoch")
	}
	if now.Before(selected.epoch.validFrom) {
		return s.commitPendingSourceWave(now, selected, summary)
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

func sameGeneration(current, base *Snapshot) bool {
	if current == nil || base == nil {
		return current == nil && base == nil
	}
	return current.Generation == base.Generation && current.Digest == base.Digest
}

func (s *networkState) commitSourceFailure(now time.Time, outcomes [4]byte, epochs [4]uint64, digests [4][32]byte) error {
	state := s.distribution
	state.observedEpochs, state.observedDigests = epochs, digests
	if err := finishWaveState(&state, now, outcomes); err != nil {
		return err
	}
	return s.commitDistribution(state)
}

func sourceConflict(valid []candidateDecision) bool {
	for first := range valid {
		for second := first + 1; second < len(valid); second++ {
			if valid[first].epoch.number == valid[second].epoch.number && valid[first].epoch.digest != valid[second].epoch.digest {
				return true
			}
		}
	}
	return false
}

func (s *networkState) finishRefresh() { s.mu.Lock(); s.refreshing = false; s.mu.Unlock() }

func containsIdentity(history [][32]byte, identity [32]byte) bool {
	for _, current := range history {
		if current == identity {
			return true
		}
	}
	return false
}
