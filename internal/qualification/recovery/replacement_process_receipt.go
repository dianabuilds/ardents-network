package recovery

import (
	"crypto/sha256"
	"encoding/binary"
)

type processFaultEvidence struct {
	Resource                                         processEvidenceRef
	Kind, State                                      string
	InvocationStartedNanos, InvocationCompletedNanos int64
	ObservedAtNanos                                  int64
	Commitment                                       [32]byte
}

type processStateEvidence struct {
	Resource        processEvidenceRef
	State           string
	ObservedAtNanos int64
	Commitment      [32]byte
}

func processFaultCommitment(value processFaultEvidence) [32]byte {
	return commitProcessReceipt("ardents-qualification-process-fault-v1\x00", value.Resource.Commitment,
		value.Kind, value.State, value.InvocationStartedNanos,
		value.InvocationCompletedNanos, value.ObservedAtNanos)
}

func processStateCommitment(value processStateEvidence) [32]byte {
	return commitProcessReceipt("ardents-qualification-process-state-v1\x00", value.Resource.Commitment,
		"", value.State, 0, 0, value.ObservedAtNanos)
}

func commitProcessReceipt(domain string, resource [32]byte, kind, state string, times ...int64) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte(domain))
	_, _ = hash.Write(resource[:])
	_, _ = hash.Write([]byte(kind + "\x00" + state + "\x00"))
	for _, value := range times {
		var encoded [8]byte
		binary.BigEndian.PutUint64(encoded[:], uint64(value))
		_, _ = hash.Write(encoded[:])
	}
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func validProcessState(value processStateEvidence, process candidateProcess) bool {
	return value.Resource == process.Host && value.State == "stopped" && value.ObservedAtNanos > 0 &&
		value.Commitment != [32]byte{} && value.Commitment == processStateCommitment(value)
}

func validProcessFault(value processFaultEvidence, process candidateProcess) bool {
	return value.Resource == process.Host && value.Kind == "stop" && value.State == "stopped" &&
		value.InvocationStartedNanos > 0 && value.InvocationCompletedNanos >= value.InvocationStartedNanos &&
		value.ObservedAtNanos >= value.InvocationStartedNanos &&
		value.ObservedAtNanos <= value.InvocationCompletedNanos && value.Commitment != [32]byte{} &&
		value.Commitment == processFaultCommitment(value)
}
