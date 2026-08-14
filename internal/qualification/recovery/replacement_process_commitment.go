package recovery

import (
	"crypto/sha256"
	"encoding/binary"
)

type processEvidenceRef struct {
	Adapter                 string
	Scope, Executable, Tree [32]byte
	Commitment              [32]byte
	Identity, Incarnation   string
}

func processRefCommitment(value processEvidenceRef) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents-qualification-host-process-v1\x00"))
	_, _ = hash.Write(value.Scope[:])
	_, _ = hash.Write(value.Executable[:])
	_, _ = hash.Write(value.Tree[:])
	for _, field := range []string{value.Adapter, value.Identity, value.Incarnation} {
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(len(field)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write([]byte(field))
	}
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func processObservationCommitment(ref processEvidenceRef, projection []byte, pid uint32,
	running bool, observedAt int64) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents-qualification-process-observation-v1\x00"))
	_, _ = hash.Write(ref.Commitment[:])
	projectionDigest := sha256.Sum256(projection)
	_, _ = hash.Write(projectionDigest[:])
	var encoded [8]byte
	binary.BigEndian.PutUint32(encoded[:4], pid)
	if running {
		encoded[4] = 1
	}
	_, _ = hash.Write(encoded[:5])
	binary.BigEndian.PutUint64(encoded[:], uint64(observedAt))
	_, _ = hash.Write(encoded[:])
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func validProcessRef(process candidateProcess, scope hostScopeEvidence) bool {
	value := process.Host
	return value.Adapter == scope.Adapter && value.Scope == scope.Commitment && value.Executable != [32]byte{} &&
		value.Tree != [32]byte{} && value.Identity != "" && value.Incarnation != "" && process.ObservedAtNanos > 0 &&
		value.Commitment != [32]byte{} && value.Commitment == processRefCommitment(value) &&
		process.AdapterProjection != "" && process.HostObservation ==
		processObservationCommitment(value, []byte(process.AdapterProjection), process.PID, true, process.ObservedAtNanos)
}

func sameProcessIncarnation(left, right candidateProcess) bool {
	return left.PID == right.PID && left.NodeID == right.NodeID && left.PublicKey == right.PublicKey &&
		left.Host == right.Host && left.AdapterProjection == right.AdapterProjection
}
