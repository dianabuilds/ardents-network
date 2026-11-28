package state

import (
	"errors"
	"time"

	networkdomain "github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/internal/distribution"
)

// checkSourceExposureCapacity refuses a changed plan before its first contact
// could grow the durable history beyond the decoder's fixed bound. A validated
// Source plan contains two distinct exposure identities.
func (s *State) checkSourceExposureCapacity() error {
	return sourceExposureCapacity(s.distribution, s.config.sourceInfo.Exposures)
}

func (s *State) beginLatestAttempt(index int) (bool, byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index > 1 {
		return false, 0, errors.New("LATEST source attempt is outside the active cycle")
	}
	state, contact, outcome, err := proposeLatestAttempt(s.distribution, index, s.config.sourceInfo.Exposures[index])
	if err != nil {
		return false, 0, err
	}
	if state.Sequence != s.distribution.Sequence {
		if err := s.commitDistribution(state); err != nil {
			return false, 0, err
		}
	}
	return contact, outcome, nil
}

func (s *State) beginDigestAttempt(source int, digest [32]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if source < 0 || source > 1 {
		return errors.New("by-digest source attempt is not available")
	}
	state, err := proposeDigestAttempt(s.distribution, source, digest, s.config.sourceInfo.Exposures[source])
	if err != nil {
		return err
	}
	return s.commitDistribution(state)
}

func (s *State) finishDigestAttempt(source int, responseCompleted bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := proposeDigestCompletion(s.distribution, source, responseCompleted)
	if err != nil {
		return err
	}
	return s.commitDistribution(state)
}

// Acquisition and Epoch/profile floors share this application's commit. These
// translations publish no independent authority and retain the journal bytes.
func acquisitionHistory(state distribution.Record) (networkdomain.Acquisition, error) {
	return networkdomain.RestoreAcquisition(networkdomain.AcquisitionFacts{Cycle: state.CycleID, Active: state.CycleActive,
		Purpose: state.CyclePurpose, Started: state.CycleStarted, Deadline: state.CycleDeadline, Seed: state.CycleSeed, Order: state.SourceOrder,
		Attempts: state.Attempts, Outcomes: state.Outcomes, RequestedDigests: state.RequestedDigests,
		ObservedEpochs: state.ObservedEpochs, ObservedDigests: state.ObservedDigests, Exposures: state.History,
		Failures: state.ConsecutiveFailures, Backoff: state.BackoffLevel, NextAttempt: state.NextAutomatic})
}

func applyAcquisition(state *distribution.Record, acquisition networkdomain.Acquisition) {
	facts := acquisition.Facts()
	state.CycleID, state.CycleActive, state.CyclePurpose = facts.Cycle, facts.Active, facts.Purpose
	state.CycleStarted, state.CycleDeadline, state.CycleSeed, state.SourceOrder = facts.Started, facts.Deadline, facts.Seed, facts.Order
	state.Attempts, state.Outcomes, state.RequestedDigests = facts.Attempts, facts.Outcomes, facts.RequestedDigests
	state.ObservedEpochs, state.ObservedDigests, state.History = facts.ObservedEpochs, facts.ObservedDigests, facts.Exposures
	state.ConsecutiveFailures, state.BackoffLevel, state.NextAutomatic = facts.Failures, facts.Backoff, facts.NextAttempt
}

func sourceExposureCapacity(state distribution.Record, exposures [2][32]byte) error {
	history, err := acquisitionHistory(state)
	if err != nil {
		return err
	}
	return history.CheckExposures(exposures)
}

func proposeLatestAttempt(state distribution.Record, source int, exposure [32]byte) (distribution.Record, bool, byte, error) {
	history, err := acquisitionHistory(state)
	if err != nil {
		return distribution.Record{}, false, 0, err
	}
	next, disposition, err := history.BeginLatest(source, exposure)
	if err != nil {
		return distribution.Record{}, false, 0, err
	}
	if disposition.Changed {
		applyAcquisition(&state, next)
		state.Sequence++
	}
	return state, disposition.Contact, disposition.Outcome, nil
}

func proposeDigestAttempt(state distribution.Record, source int, digest, exposure [32]byte) (distribution.Record, error) {
	history, err := acquisitionHistory(state)
	if err != nil {
		return distribution.Record{}, err
	}
	next, err := history.BeginDigest(source, digest, exposure)
	if err != nil {
		return distribution.Record{}, err
	}
	applyAcquisition(&state, next)
	state.Sequence++
	return state, nil
}

func proposeDigestCompletion(state distribution.Record, source int, responseCompleted bool) (distribution.Record, error) {
	history, err := acquisitionHistory(state)
	if err != nil {
		return distribution.Record{}, err
	}
	next, err := history.CompleteDigest(source, responseCompleted)
	if err != nil {
		return distribution.Record{}, err
	}
	applyAcquisition(&state, next)
	state.Sequence++
	return state, nil
}

func proposeSourceCycle(state distribution.Record, now time.Time, seed [32]byte) (distribution.Record, bool, error) {
	history, err := acquisitionHistory(state)
	if err != nil {
		return distribution.Record{}, false, err
	}
	next, changed, expired, err := history.Start(now, seed)
	if err != nil {
		if errors.Is(err, networkdomain.ErrAcquisitionBackoff) {
			return distribution.Record{}, false, errors.Join(errRefreshUnavailable, err)
		}
		return distribution.Record{}, false, err
	}
	if changed {
		applyAcquisition(&state, next)
		state.Sequence++
		if !expired && !history.Facts().Active {
			state.TrustedTimeFloor = max(state.TrustedTimeFloor, now.Unix())
		}
	}
	return state, expired, nil
}

// finishWaveState translates a Network transition into the same copied journal.
// No outcome becomes visible until the application commits it.
func finishWaveState(state *distribution.Record, now time.Time, outcomes [4]byte) error {
	history, err := acquisitionHistory(*state)
	if err != nil {
		return err
	}
	next, err := history.Finish(now, outcomes)
	if err != nil {
		return err
	}
	applyAcquisition(state, next)
	state.Sequence++
	return nil
}
