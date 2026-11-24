package state

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/network/canonicalcursor"
)

func decodeDistributionState(raw []byte) (distributionState, error) {
	d := canonicalcursor.NewDecoder(raw)
	magic, err := d.Bytes(8)
	if err != nil || string(magic) != "ARDS1D4\x00" {
		return distributionState{}, errors.New("distribution state magic is invalid")
	}
	var state distributionState
	if err := decodeDistributionHeader(&d, &state); err != nil {
		return state, err
	}
	if err := decodeDistributionCycle(&d, &state); err != nil {
		return state, err
	}
	if err := decodeDistributionEvidence(&d, &state); err != nil {
		return state, err
	}
	if !d.Done() {
		return state, errors.New("distribution state has trailing bytes")
	}
	return state, nil
}

func decodeDistributionHeader(d *canonicalcursor.Decoder, state *distributionState) error {
	var err error
	if state.sequence, err = d.Uint64(); err != nil {
		return err
	}
	if state.epochFloor, err = d.Uint64(); err != nil {
		return err
	}
	digest, err := d.Bytes(32)
	if err != nil {
		return err
	}
	copy(state.epochDigest[:], digest)
	floor, err := d.Uint64()
	if err != nil {
		return err
	}
	state.trustedTimeFloor = int64(floor)
	conflict, err := d.Byte()
	if err != nil || conflict > 1 {
		return errors.New("distribution conflict flag is invalid")
	}
	state.conflicting = conflict == 1
	if state.consecutiveFailures, err = d.Uint64(); err != nil {
		return err
	}
	if state.backoffLevel, err = d.Byte(); err != nil || state.backoffLevel > 5 {
		return errors.New("distribution backoff level is invalid")
	}
	next, err := d.Uint64()
	if err != nil {
		return err
	}
	state.nextAutomatic = int64(next)
	count, err := d.Byte()
	if err != nil || count > maximumSourceExposureHistory {
		return errors.New("distribution history count is invalid")
	}
	for range int(count) {
		value, readErr := d.Bytes(32)
		if readErr != nil {
			return readErr
		}
		var identity [32]byte
		copy(identity[:], value)
		state.history = append(state.history, identity)
	}
	return nil
}

func decodeDistributionCycle(d *canonicalcursor.Decoder, state *distributionState) error {
	var err error
	if state.cycleID, err = d.Uint64(); err != nil {
		return err
	}
	active, err := d.Byte()
	if err != nil || active > 1 {
		return errors.New("distribution cycle flag is invalid")
	}
	state.cycleActive = active == 1
	if state.cyclePurpose, err = d.Byte(); err != nil || state.cyclePurpose > sourceCyclePurposeRefresh {
		return errors.New("distribution cycle purpose is invalid")
	}
	started, err := d.Uint64()
	if err != nil {
		return err
	}
	deadline, err := d.Uint64()
	if err != nil {
		return err
	}
	state.cycleStarted, state.cycleDeadline = int64(started), int64(deadline)
	if state.cycleActive && (state.cyclePurpose != sourceCyclePurposeRefresh || state.cycleStarted <= 0 || state.cycleDeadline <= state.cycleStarted) {
		return errors.New("active distribution cycle metadata is incomplete")
	}
	statuses, err := d.Bytes(len(state.attempts))
	if err != nil {
		return err
	}
	copy(state.attempts[:], statuses)
	for _, status := range state.attempts {
		if status > sourceAttemptFailed {
			return errors.New("distribution attempt status is invalid")
		}
	}
	outcomes, err := d.Bytes(len(state.outcomes))
	if err != nil {
		return err
	}
	copy(state.outcomes[:], outcomes)
	for _, outcome := range state.outcomes {
		if outcome > sourceOutcomeInternal {
			return errors.New("distribution source outcome is invalid")
		}
	}
	for index := range state.requestedDigests {
		digest, readErr := d.Bytes(32)
		if readErr != nil {
			return readErr
		}
		copy(state.requestedDigests[index][:], digest)
		if (state.requestedDigests[index] == [32]byte{}) != (state.attempts[digestAttemptSlot(index)] == sourceAttemptNotStarted) {
			return errors.New("BY_DIGEST attempt lacks its exact selector")
		}
	}
	return nil
}

func decodeDistributionEvidence(d *canonicalcursor.Decoder, state *distributionState) error {
	var err error
	for index := range state.observedEpochs {
		if state.observedEpochs[index], err = d.Uint64(); err != nil {
			return err
		}
		digest, readErr := d.Bytes(32)
		if readErr != nil {
			return readErr
		}
		copy(state.observedDigests[index][:], digest)
		if (state.observedEpochs[index] == 0) != (state.observedDigests[index] == [32]byte{}) {
			return errors.New("observed source candidate identity is incomplete")
		}
	}
	pending, err := d.Bytes(32)
	if err != nil {
		return err
	}
	copy(state.pendingDigest[:], pending)
	pendingAt, err := d.Uint64()
	if err != nil {
		return err
	}
	state.pendingValidFrom = int64(pendingAt)
	if (state.pendingDigest == [32]byte{}) != (state.pendingValidFrom == 0) {
		return errors.New("distribution pending identity is incomplete")
	}
	seed, err := d.Bytes(32)
	if err != nil {
		return err
	}
	copy(state.cycleSeed[:], seed)
	order, err := d.Bytes(2)
	if err != nil {
		return err
	}
	copy(state.sourceOrder[:], order)
	if state.cycleID > 0 && (state.sourceOrder[0] > 1 || state.sourceOrder[1] > 1 || state.sourceOrder[0] == state.sourceOrder[1]) {
		return errors.New("distribution source order is invalid")
	}
	return nil
}
