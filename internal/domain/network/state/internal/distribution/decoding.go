package distribution

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/network/canonicalcursor"
)

const recordMagic = "ARDS1D4\x00"

// Decode validates the bounded journal grammar and detaches its fields.
func Decode(raw []byte) (Record, error) {
	d := canonicalcursor.NewDecoder(raw)
	magic, err := d.Bytes(8)
	if err != nil || string(magic) != recordMagic {
		return Record{}, errors.New("distribution state magic is invalid")
	}
	var state Record
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

func decodeDistributionHeader(d *canonicalcursor.Decoder, state *Record) error {
	var err error
	if state.Sequence, err = d.Uint64(); err != nil {
		return err
	}
	if state.EpochFloor, err = d.Uint64(); err != nil {
		return err
	}
	digest, err := d.Bytes(32)
	if err != nil {
		return err
	}
	copy(state.EpochDigest[:], digest)
	floor, err := d.Uint64()
	if err != nil {
		return err
	}
	state.TrustedTimeFloor = int64(floor)
	conflict, err := d.Byte()
	if err != nil || conflict > 1 {
		return errors.New("distribution conflict flag is invalid")
	}
	state.Conflicting = conflict == 1
	if state.ConsecutiveFailures, err = d.Uint64(); err != nil {
		return err
	}
	if state.BackoffLevel, err = d.Byte(); err != nil || state.BackoffLevel > 5 {
		return errors.New("distribution backoff level is invalid")
	}
	next, err := d.Uint64()
	if err != nil {
		return err
	}
	state.NextAutomatic = int64(next)
	count, err := d.Byte()
	if err != nil || count > MaximumExposures {
		return errors.New("distribution history count is invalid")
	}
	for range int(count) {
		value, readErr := d.Bytes(32)
		if readErr != nil {
			return readErr
		}
		var identity [32]byte
		copy(identity[:], value)
		state.History = append(state.History, identity)
	}
	return nil
}

func decodeDistributionCycle(d *canonicalcursor.Decoder, state *Record) error {
	var err error
	if state.CycleID, err = d.Uint64(); err != nil {
		return err
	}
	active, err := d.Byte()
	if err != nil || active > 1 {
		return errors.New("distribution cycle flag is invalid")
	}
	state.CycleActive = active == 1
	if state.CyclePurpose, err = d.Byte(); err != nil || state.CyclePurpose > Refresh {
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
	state.CycleStarted, state.CycleDeadline = int64(started), int64(deadline)
	if state.CycleActive && (state.CyclePurpose != Refresh || state.CycleStarted <= 0 || state.CycleDeadline <= state.CycleStarted) {
		return errors.New("active distribution cycle metadata is incomplete")
	}
	statuses, err := d.Bytes(len(state.Attempts))
	if err != nil {
		return err
	}
	copy(state.Attempts[:], statuses)
	for _, status := range state.Attempts {
		if status > AttemptFailed {
			return errors.New("distribution attempt status is invalid")
		}
	}
	outcomes, err := d.Bytes(len(state.Outcomes))
	if err != nil {
		return err
	}
	copy(state.Outcomes[:], outcomes)
	for _, outcome := range state.Outcomes {
		if outcome > OutcomeInternal {
			return errors.New("distribution source outcome is invalid")
		}
	}
	for index := range state.RequestedDigests {
		digest, readErr := d.Bytes(32)
		if readErr != nil {
			return readErr
		}
		copy(state.RequestedDigests[index][:], digest)
		if (state.RequestedDigests[index] == [32]byte{}) != (state.Attempts[ByDigestSlot(index)] == AttemptNotStarted) {
			return errors.New("BY_DIGEST attempt lacks its exact selector")
		}
	}
	return nil
}

func decodeDistributionEvidence(d *canonicalcursor.Decoder, state *Record) error {
	var err error
	for index := range state.ObservedEpochs {
		if state.ObservedEpochs[index], err = d.Uint64(); err != nil {
			return err
		}
		digest, readErr := d.Bytes(32)
		if readErr != nil {
			return readErr
		}
		copy(state.ObservedDigests[index][:], digest)
		if (state.ObservedEpochs[index] == 0) != (state.ObservedDigests[index] == [32]byte{}) {
			return errors.New("observed source candidate identity is incomplete")
		}
	}
	pending, err := d.Bytes(32)
	if err != nil {
		return err
	}
	copy(state.PendingDigest[:], pending)
	pendingAt, err := d.Uint64()
	if err != nil {
		return err
	}
	state.PendingValidFrom = int64(pendingAt)
	if (state.PendingDigest == [32]byte{}) != (state.PendingValidFrom == 0) {
		return errors.New("distribution pending identity is incomplete")
	}
	seed, err := d.Bytes(32)
	if err != nil {
		return err
	}
	copy(state.CycleSeed[:], seed)
	order, err := d.Bytes(2)
	if err != nil {
		return err
	}
	copy(state.SourceOrder[:], order)
	if state.CycleID > 0 && (state.SourceOrder[0] > 1 || state.SourceOrder[1] > 1 || state.SourceOrder[0] == state.SourceOrder[1]) {
		return errors.New("distribution source order is invalid")
	}
	return nil
}
