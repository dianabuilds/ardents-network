package distribution

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// Encode emits the existing canonical journal record.
func Encode(state Record) []byte {
	raw := make([]byte, 0, 160+len(state.History)*32)
	raw = append(raw, recordMagic...)
	raw = binary.BigEndian.AppendUint64(raw, state.Sequence)
	raw = binary.BigEndian.AppendUint64(raw, state.EpochFloor)
	raw = append(raw, state.EpochDigest[:]...)
	raw = binary.BigEndian.AppendUint64(raw, uint64(state.TrustedTimeFloor))
	if state.Conflicting {
		raw = append(raw, 1)
	} else {
		raw = append(raw, 0)
	}
	raw = binary.BigEndian.AppendUint64(raw, state.ConsecutiveFailures)
	raw = append(raw, state.BackoffLevel)
	raw = binary.BigEndian.AppendUint64(raw, uint64(state.NextAutomatic))
	raw = append(raw, byte(len(state.History)))
	for _, identity := range state.History {
		raw = append(raw, identity[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, state.CycleID)
	if state.CycleActive {
		raw = append(raw, 1)
	} else {
		raw = append(raw, 0)
	}
	raw = append(raw, state.CyclePurpose)
	raw = binary.BigEndian.AppendUint64(raw, uint64(state.CycleStarted))
	raw = binary.BigEndian.AppendUint64(raw, uint64(state.CycleDeadline))
	raw = append(raw, state.Attempts[:]...)
	raw = append(raw, state.Outcomes[:]...)
	for _, digest := range state.RequestedDigests {
		raw = append(raw, digest[:]...)
	}
	for index, epoch := range state.ObservedEpochs {
		raw = binary.BigEndian.AppendUint64(raw, epoch)
		raw = append(raw, state.ObservedDigests[index][:]...)
	}
	raw = append(raw, state.PendingDigest[:]...)
	raw = binary.BigEndian.AppendUint64(raw, uint64(state.PendingValidFrom))
	raw = append(raw, state.CycleSeed[:]...)
	raw = append(raw, state.SourceOrder[:]...)
	return raw
}

// Digest identifies the exact canonical journal bytes.
func Digest(raw []byte) string {
	value := sha256.Sum256(append([]byte("ardents-h3-distribution-state-v1\x00"), raw...))
	return fmt.Sprintf("%x", value)
}
