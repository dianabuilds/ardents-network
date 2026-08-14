package recovery

import (
	"crypto/sha256"
	"encoding/binary"
)

func cleanupObservationCommitment(value cleanup) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents-qualification-cleanup-observation-v1\x00"))
	_, _ = hash.Write(value.Scope[:])
	_, _ = hash.Write([]byte(value.Adapter))
	projection := sha256.Sum256(value.AdapterProjection)
	_, _ = hash.Write(projection[:])
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(value.ObservedAtNanos))
	_, _ = hash.Write(encoded[:])
	binary.BigEndian.PutUint32(encoded[:4], value.OwnedResources)
	_, _ = hash.Write(encoded[:4])
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func validCleanupObservation(value cleanup, scope hostScopeEvidence) bool {
	return value.Adapter == scope.Adapter && value.Scope == scope.Commitment && value.ObservedAtNanos > 0 &&
		value.OwnedResources == 0 && len(value.AdapterProjection) > 0 && len(value.AdapterProjection) <= 64<<10 &&
		value.Observation != [32]byte{} && value.Observation == cleanupObservationCommitment(value)
}
