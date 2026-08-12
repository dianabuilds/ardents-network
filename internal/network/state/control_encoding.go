package state

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

func encodeDistributionState(state distributionState) []byte {
	raw := make([]byte, 0, 160+len(state.history)*32)
	raw = append(raw, "ARDS1D4\x00"...)
	raw = binary.BigEndian.AppendUint64(raw, state.sequence)
	raw = binary.BigEndian.AppendUint64(raw, state.epochFloor)
	raw = append(raw, state.epochDigest[:]...)
	raw = binary.BigEndian.AppendUint64(raw, uint64(state.trustedTimeFloor))
	if state.conflicting {
		raw = append(raw, 1)
	} else {
		raw = append(raw, 0)
	}
	raw = binary.BigEndian.AppendUint64(raw, state.consecutiveFailures)
	raw = append(raw, state.backoffLevel)
	raw = binary.BigEndian.AppendUint64(raw, uint64(state.nextAutomatic))
	raw = append(raw, byte(len(state.history)))
	for _, identity := range state.history {
		raw = append(raw, identity[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, state.cycleID)
	if state.cycleActive {
		raw = append(raw, 1)
	} else {
		raw = append(raw, 0)
	}
	raw = append(raw, state.cyclePurpose)
	raw = binary.BigEndian.AppendUint64(raw, uint64(state.cycleStarted))
	raw = binary.BigEndian.AppendUint64(raw, uint64(state.cycleDeadline))
	raw = append(raw, state.attempts[:]...)
	raw = append(raw, state.outcomes[:]...)
	for _, digest := range state.requestedDigests {
		raw = append(raw, digest[:]...)
	}
	for index, epoch := range state.observedEpochs {
		raw = binary.BigEndian.AppendUint64(raw, epoch)
		raw = append(raw, state.observedDigests[index][:]...)
	}
	raw = append(raw, state.pendingDigest[:]...)
	raw = binary.BigEndian.AppendUint64(raw, uint64(state.pendingValidFrom))
	raw = append(raw, state.cycleSeed[:]...)
	raw = append(raw, state.sourceOrder[:]...)
	return raw
}

func decodeDistributionState(raw []byte) (distributionState, error) {
	d := newDecoder(raw)
	magic, err := d.bytes(8)
	if err != nil || string(magic) != "ARDS1D4\x00" {
		return distributionState{}, errors.New("distribution state magic is invalid")
	}
	var state distributionState
	if state.sequence, err = d.uint64(); err != nil {
		return state, err
	}
	if state.epochFloor, err = d.uint64(); err != nil {
		return state, err
	}
	digest, err := d.bytes(32)
	if err != nil {
		return state, err
	}
	copy(state.epochDigest[:], digest)
	floor, err := d.uint64()
	if err != nil {
		return state, err
	}
	state.trustedTimeFloor = int64(floor)
	conflict, err := d.byte()
	if err != nil || conflict > 1 {
		return state, errors.New("distribution conflict flag is invalid")
	}
	state.conflicting = conflict == 1
	if state.consecutiveFailures, err = d.uint64(); err != nil {
		return state, err
	}
	if state.backoffLevel, err = d.byte(); err != nil || state.backoffLevel > 5 {
		return state, errors.New("distribution backoff level is invalid")
	}
	next, err := d.uint64()
	if err != nil {
		return state, err
	}
	state.nextAutomatic = int64(next)
	count, err := d.byte()
	if err != nil || count > 2 {
		return state, errors.New("distribution history count is invalid")
	}
	for range int(count) {
		value, readErr := d.bytes(32)
		if readErr != nil {
			return state, readErr
		}
		var id [32]byte
		copy(id[:], value)
		state.history = append(state.history, id)
	}
	if state.cycleID, err = d.uint64(); err != nil {
		return state, err
	}
	active, err := d.byte()
	if err != nil || active > 1 {
		return state, errors.New("distribution cycle flag is invalid")
	}
	state.cycleActive = active == 1
	if state.cyclePurpose, err = d.byte(); err != nil || state.cyclePurpose > 1 {
		return state, errors.New("distribution cycle purpose is invalid")
	}
	started, err := d.uint64()
	if err != nil {
		return state, err
	}
	deadline, err := d.uint64()
	if err != nil {
		return state, err
	}
	state.cycleStarted, state.cycleDeadline = int64(started), int64(deadline)
	if state.cycleActive && (state.cyclePurpose != 1 || state.cycleStarted <= 0 || state.cycleDeadline <= state.cycleStarted) {
		return state, errors.New("active distribution cycle metadata is incomplete")
	}
	statuses, err := d.bytes(len(state.attempts))
	if err != nil {
		return state, err
	}
	copy(state.attempts[:], statuses)
	for _, status := range state.attempts {
		if status > 3 {
			return state, errors.New("distribution attempt status is invalid")
		}
	}
	outcomes, err := d.bytes(len(state.outcomes))
	if err != nil {
		return state, err
	}
	copy(state.outcomes[:], outcomes)
	for _, outcome := range state.outcomes {
		if outcome > sourceOutcomeInternal {
			return state, errors.New("distribution source outcome is invalid")
		}
	}
	for index := range state.requestedDigests {
		digest, readErr := d.bytes(32)
		if readErr != nil {
			return state, readErr
		}
		copy(state.requestedDigests[index][:], digest)
		if isZero32(state.requestedDigests[index]) != (state.attempts[index+2] == 0) {
			return state, errors.New("BY_DIGEST attempt lacks its exact selector")
		}
	}
	for index := range state.observedEpochs {
		if state.observedEpochs[index], err = d.uint64(); err != nil {
			return state, err
		}
		digest, readErr := d.bytes(32)
		if readErr != nil {
			return state, readErr
		}
		copy(state.observedDigests[index][:], digest)
		if (state.observedEpochs[index] == 0) != isZero32(state.observedDigests[index]) {
			return state, errors.New("observed source candidate identity is incomplete")
		}
	}
	pending, err := d.bytes(32)
	if err != nil {
		return state, err
	}
	copy(state.pendingDigest[:], pending)
	pendingAt, err := d.uint64()
	if err != nil {
		return state, err
	}
	state.pendingValidFrom = int64(pendingAt)
	if isZero32(state.pendingDigest) != (state.pendingValidFrom == 0) {
		return state, errors.New("distribution pending identity is incomplete")
	}
	seed, err := d.bytes(32)
	if err != nil {
		return state, err
	}
	copy(state.cycleSeed[:], seed)
	order, err := d.bytes(2)
	if err != nil {
		return state, err
	}
	copy(state.sourceOrder[:], order)
	if state.cycleID > 0 && (state.sourceOrder[0] > 1 || state.sourceOrder[1] > 1 || state.sourceOrder[0] == state.sourceOrder[1]) {
		return state, errors.New("distribution source order is invalid")
	}
	if !d.done() {
		return state, errors.New("distribution state has trailing bytes")
	}
	return state, nil
}

func distributionDigest(raw []byte) string {
	value := sha256.Sum256(append([]byte("ardents-h3-distribution-state-v1\x00"), raw...))
	return fmt.Sprintf("%x", value)
}

func trustedNow(config config, state distributionState) (time.Time, error) {
	now := config.clock().UTC()
	monotonic := config.anchorWall.Add(time.Since(config.anchorMono))
	if monotonic.After(now) {
		now = monotonic
	}
	observation := config.observe().UTC()
	if observation.IsZero() || now.Sub(observation).Abs() > 2*time.Second {
		return time.Time{}, errClockUncertain
	}
	if now.Unix()+2 < state.trustedTimeFloor {
		return time.Time{}, errClockUncertain
	}
	if now.Unix() < state.trustedTimeFloor {
		return time.Unix(state.trustedTimeFloor, 0).UTC(), nil
	}
	return now, nil
}
