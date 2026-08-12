package state

import "errors"

func decodeDistributionState(raw []byte) (distributionState, error) {
	d := newDecoder(raw)
	magic, err := d.bytes(8)
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
	if !d.done() {
		return state, errors.New("distribution state has trailing bytes")
	}
	return state, nil
}

func decodeDistributionHeader(d *decoder, state *distributionState) error {
	var err error
	if state.sequence, err = d.uint64(); err != nil {
		return err
	}
	if state.epochFloor, err = d.uint64(); err != nil {
		return err
	}
	digest, err := d.bytes(32)
	if err != nil {
		return err
	}
	copy(state.epochDigest[:], digest)
	floor, err := d.uint64()
	if err != nil {
		return err
	}
	state.trustedTimeFloor = int64(floor)
	conflict, err := d.byte()
	if err != nil || conflict > 1 {
		return errors.New("distribution conflict flag is invalid")
	}
	state.conflicting = conflict == 1
	if state.consecutiveFailures, err = d.uint64(); err != nil {
		return err
	}
	if state.backoffLevel, err = d.byte(); err != nil || state.backoffLevel > 5 {
		return errors.New("distribution backoff level is invalid")
	}
	next, err := d.uint64()
	if err != nil {
		return err
	}
	state.nextAutomatic = int64(next)
	count, err := d.byte()
	if err != nil || count > 2 {
		return errors.New("distribution history count is invalid")
	}
	for range int(count) {
		value, readErr := d.bytes(32)
		if readErr != nil {
			return readErr
		}
		var identity [32]byte
		copy(identity[:], value)
		state.history = append(state.history, identity)
	}
	return nil
}

func decodeDistributionCycle(d *decoder, state *distributionState) error {
	var err error
	if state.cycleID, err = d.uint64(); err != nil {
		return err
	}
	active, err := d.byte()
	if err != nil || active > 1 {
		return errors.New("distribution cycle flag is invalid")
	}
	state.cycleActive = active == 1
	if state.cyclePurpose, err = d.byte(); err != nil || state.cyclePurpose > 1 {
		return errors.New("distribution cycle purpose is invalid")
	}
	started, err := d.uint64()
	if err != nil {
		return err
	}
	deadline, err := d.uint64()
	if err != nil {
		return err
	}
	state.cycleStarted, state.cycleDeadline = int64(started), int64(deadline)
	if state.cycleActive && (state.cyclePurpose != 1 || state.cycleStarted <= 0 || state.cycleDeadline <= state.cycleStarted) {
		return errors.New("active distribution cycle metadata is incomplete")
	}
	statuses, err := d.bytes(len(state.attempts))
	if err != nil {
		return err
	}
	copy(state.attempts[:], statuses)
	for _, status := range state.attempts {
		if status > 3 {
			return errors.New("distribution attempt status is invalid")
		}
	}
	outcomes, err := d.bytes(len(state.outcomes))
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
		digest, readErr := d.bytes(32)
		if readErr != nil {
			return readErr
		}
		copy(state.requestedDigests[index][:], digest)
		if isZero32(state.requestedDigests[index]) != (state.attempts[index+2] == 0) {
			return errors.New("BY_DIGEST attempt lacks its exact selector")
		}
	}
	return nil
}

func decodeDistributionEvidence(d *decoder, state *distributionState) error {
	var err error
	for index := range state.observedEpochs {
		if state.observedEpochs[index], err = d.uint64(); err != nil {
			return err
		}
		digest, readErr := d.bytes(32)
		if readErr != nil {
			return readErr
		}
		copy(state.observedDigests[index][:], digest)
		if (state.observedEpochs[index] == 0) != isZero32(state.observedDigests[index]) {
			return errors.New("observed source candidate identity is incomplete")
		}
	}
	pending, err := d.bytes(32)
	if err != nil {
		return err
	}
	copy(state.pendingDigest[:], pending)
	pendingAt, err := d.uint64()
	if err != nil {
		return err
	}
	state.pendingValidFrom = int64(pendingAt)
	if isZero32(state.pendingDigest) != (state.pendingValidFrom == 0) {
		return errors.New("distribution pending identity is incomplete")
	}
	seed, err := d.bytes(32)
	if err != nil {
		return err
	}
	copy(state.cycleSeed[:], seed)
	order, err := d.bytes(2)
	if err != nil {
		return err
	}
	copy(state.sourceOrder[:], order)
	if state.cycleID > 0 && (state.sourceOrder[0] > 1 || state.sourceOrder[1] > 1 || state.sourceOrder[0] == state.sourceOrder[1]) {
		return errors.New("distribution source order is invalid")
	}
	return nil
}
