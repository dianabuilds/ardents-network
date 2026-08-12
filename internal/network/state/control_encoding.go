package state

import (
	"crypto/sha256"
	"encoding/binary"
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
