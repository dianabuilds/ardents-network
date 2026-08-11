package networkstate

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"
)

func (s *store) startSourceWave(now time.Time) ([2]int, time.Time, error) {
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
	seed := s.config.orderSeed
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

func (s *store) completeSourceWave(now time.Time, base *Snapshot, results []sourceResult) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() { s.refreshing = false }()
	if s.closed {
		return Snapshot{}, errors.New("network state is closed")
	}
	if !sameGeneration(s.current, base) {
		return Snapshot{}, errors.New("network state changed during the finite source wave")
	}
	valid := make([]candidateDecision, 0, 2)
	outcomes := [4]byte{}
	observedEpochs := [4]uint64{}
	observedDigests := [4][32]byte{}
	var collisionErr error
	for _, result := range results {
		for index, outcome := range result.observations {
			if outcome != 0 {
				outcomes[index] = outcome
			}
		}
		if errors.Is(result.err, errSourceRoleCollision) && collisionErr == nil {
			collisionErr = result.err
		}
		if result.err == nil {
			valid = append(valid, result.decision)
			observedEpochs[result.slot] = result.decision.epoch.number
			observedDigests[result.slot] = result.decision.epoch.digest
		}
	}
	if collisionErr != nil {
		state := s.distribution
		state.observedEpochs, state.observedDigests = observedEpochs, observedDigests
		if err := finishWaveState(&state, now, outcomes); err != nil {
			return Snapshot{}, err
		}
		state.conflicting = true
		if err := s.commitDistribution(state); err != nil {
			return Snapshot{}, err
		}
		return Snapshot{}, collisionErr
	}
	if sourceConflict(valid) {
		state := s.distribution
		state.observedEpochs, state.observedDigests = observedEpochs, observedDigests
		if err := finishWaveState(&state, now, outcomes); err != nil {
			return Snapshot{}, err
		}
		state.conflicting = true
		if err := s.commitDistribution(state); err != nil {
			return Snapshot{}, err
		}
		return Snapshot{}, errors.New("sources exposed threshold-valid conflicting Epochs")
	}
	if len(valid) == 0 {
		if err := s.commitSourceFailure(now, outcomes, observedEpochs, observedDigests); err != nil {
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
	selected := valid[0]
	for _, candidate := range valid[1:] {
		if candidate.epoch.number > selected.epoch.number {
			selected = candidate
		}
	}
	if s.pendingDecision != nil && selected.epoch.number == s.pendingDecision.epoch.number && selected.epoch.digest != s.pendingDecision.epoch.digest {
		state := s.distribution
		state.observedEpochs, state.observedDigests = observedEpochs, observedDigests
		if err := finishWaveState(&state, now, outcomes); err != nil {
			return Snapshot{}, err
		}
		state.conflicting = true
		if err := s.commitDistribution(state); err != nil {
			return Snapshot{}, err
		}
		return Snapshot{}, errors.New("source Epoch conflicts with the durable pending Epoch")
	}
	if now.Before(selected.epoch.validFrom) {
		newPending := s.pendingDecision == nil
		if newPending {
			if err := stageGeneration(s.config.root, selected); err != nil {
				return Snapshot{}, err
			}
		}
		state := s.distribution
		state.observedEpochs, state.observedDigests = observedEpochs, observedDigests
		if err := finishWaveState(&state, now, outcomes); err != nil {
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
	state := s.distribution
	state.observedEpochs, state.observedDigests = observedEpochs, observedDigests
	if err := finishWaveState(&state, now, outcomes); err != nil {
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

func sameGeneration(current, base *Snapshot) bool {
	if current == nil || base == nil {
		return current == nil && base == nil
	}
	return current.Generation == base.Generation && current.Digest == base.Digest
}

func (s *store) commitSourceFailure(now time.Time, outcomes [4]byte, epochs [4]uint64, digests [4][32]byte) error {
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

func (s *store) finishRefresh() { s.mu.Lock(); s.refreshing = false; s.mu.Unlock() }

func containsIdentity(history [][32]byte, identity [32]byte) bool {
	for _, current := range history {
		if current == identity {
			return true
		}
	}
	return false
}
