package recoverysmoke

import (
	"crypto/sha256"
	"encoding/binary"
)

type processFaultEvidence struct {
	Resource                                         processEvidenceRef
	Kind                                             processFaultKind
	State                                            processState
	InvocationStartedNanos, InvocationCompletedNanos int64
	ObservedAtNanos                                  int64
	Commitment                                       [32]byte
}

type processStateEvidence struct {
	Resource        processEvidenceRef
	State           processState
	ObservedAtNanos int64
	Commitment      [32]byte
}

func freezeProcessFault(value processFaultReceipt) processFaultEvidence {
	result := processFaultEvidence{Resource: value.Ref, Kind: value.Kind, State: value.State,
		InvocationStartedNanos:   value.InvocationStartedNanos,
		InvocationCompletedNanos: value.InvocationCompletedNanos, ObservedAtNanos: value.ObservedAtNanos}
	result.Commitment = processFaultCommitment(result)
	return result
}

func freezeProcessState(value processStateObservation) processStateEvidence {
	result := processStateEvidence{Resource: value.Ref, State: value.State, ObservedAtNanos: value.ObservedAtNanos}
	result.Commitment = processStateCommitment(result)
	return result
}

func processFaultCommitment(value processFaultEvidence) [32]byte {
	return commitProcessReceipt("ardents-qualification-process-fault-v1\x00", value.Resource.Commitment,
		string(value.Kind), string(value.State), value.InvocationStartedNanos,
		value.InvocationCompletedNanos, value.ObservedAtNanos)
}

func processStateCommitment(value processStateEvidence) [32]byte {
	return commitProcessReceipt("ardents-qualification-process-state-v1\x00", value.Resource.Commitment,
		"", string(value.State), 0, 0, value.ObservedAtNanos)
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
