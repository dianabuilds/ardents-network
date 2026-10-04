package state

import (
	"errors"
	networkdomain "github.com/dianabuilds/ardents-network/internal/successor/network"
	"time"
)

// Acquisition and Epoch/profile floors share this application's commit. These
// translations publish no independent authority and retain the journal bytes.
func acquisitionHistory(state distributionState) (networkdomain.Acquisition, error) {
	return networkdomain.RestoreAcquisition(networkdomain.AcquisitionFacts{Cycle: state.cycleID, Active: state.cycleActive,
		Purpose: state.cyclePurpose, Started: state.cycleStarted, Deadline: state.cycleDeadline, Seed: state.cycleSeed, Order: state.sourceOrder,
		Attempts: state.attempts, Outcomes: state.outcomes, RequestedDigests: state.requestedDigests,
		ObservedEpochs: state.observedEpochs, ObservedDigests: state.observedDigests, Exposures: state.history,
		Failures: state.consecutiveFailures, Backoff: state.backoffLevel, NextAttempt: state.nextAutomatic})
}

func applyAcquisition(state *distributionState, acquisition networkdomain.Acquisition) {
	facts := acquisition.Facts()
	state.cycleID, state.cycleActive, state.cyclePurpose = facts.Cycle, facts.Active, facts.Purpose
	state.cycleStarted, state.cycleDeadline, state.cycleSeed, state.sourceOrder = facts.Started, facts.Deadline, facts.Seed, facts.Order
	state.attempts, state.outcomes, state.requestedDigests = facts.Attempts, facts.Outcomes, facts.RequestedDigests
	state.observedEpochs, state.observedDigests, state.history = facts.ObservedEpochs, facts.ObservedDigests, facts.Exposures
	state.consecutiveFailures, state.backoffLevel, state.nextAutomatic = facts.Failures, facts.Backoff, facts.NextAttempt
}

func sourceExposureCapacity(state distributionState, exposures [2][32]byte) error {
	history, err := acquisitionHistory(state)
	if err != nil {
		return err
	}
	return history.CheckExposures(exposures)
}

func proposeLatestAttempt(state distributionState, source int, exposure [32]byte) (distributionState, bool, byte, error) {
	history, err := acquisitionHistory(state)
	if err != nil {
		return distributionState{}, false, 0, err
	}
	next, disposition, err := history.BeginLatest(source, exposure)
	if err != nil {
		return distributionState{}, false, 0, err
	}
	if disposition.Changed {
		applyAcquisition(&state, next)
		state.sequence++
	}
	return state, disposition.Contact, disposition.Outcome, nil
}

func proposeDigestAttempt(state distributionState, source int, digest, exposure [32]byte) (distributionState, error) {
	history, err := acquisitionHistory(state)
	if err != nil {
		return distributionState{}, err
	}
	next, err := history.BeginDigest(source, digest, exposure)
	if err != nil {
		return distributionState{}, err
	}
	applyAcquisition(&state, next)
	state.sequence++
	return state, nil
}

func proposeDigestCompletion(state distributionState, source int, responseCompleted bool) (distributionState, error) {
	history, err := acquisitionHistory(state)
	if err != nil {
		return distributionState{}, err
	}
	next, err := history.CompleteDigest(source, responseCompleted)
	if err != nil {
		return distributionState{}, err
	}
	applyAcquisition(&state, next)
	state.sequence++
	return state, nil
}

func proposeSourceCycle(state distributionState, now time.Time, seed [32]byte) (distributionState, bool, error) {
	history, err := acquisitionHistory(state)
	if err != nil {
		return distributionState{}, false, err
	}
	next, changed, expired, err := history.Start(now, seed)
	if err != nil {
		if errors.Is(err, networkdomain.ErrAcquisitionBackoff) {
			return distributionState{}, false, errors.Join(errRefreshUnavailable, err)
		}
		return distributionState{}, false, err
	}
	if changed {
		applyAcquisition(&state, next)
		state.sequence++
		if !expired && !history.Facts().Active {
			state.trustedTimeFloor = max(state.trustedTimeFloor, now.Unix())
		}
	}
	return state, expired, nil
}
