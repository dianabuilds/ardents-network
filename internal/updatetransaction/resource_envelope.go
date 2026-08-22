package updatetransaction

import "errors"

const (
	journalV1Bytes      = recordHeaderBytes + journalBodyBytes
	journalV1EntryCount = 9
	resourceObjectCount = 15
)

// resourceObservation is a single immutable capacity observation made while
// the permanent root lock is held. It is deliberately private: callers cannot
// turn a point-in-time filesystem observation into a reservation.
type resourceObservation struct {
	allocationUnit uint64
	availableBytes uint64
	availableItems uint64
	itemsKnown     bool
}

func requireResourceEnvelope(observation resourceObservation, artifact, manifest []byte, successorCurrent []byte) error {
	if observation.allocationUnit == 0 || len(artifact) == 0 || len(manifest) == 0 || len(successorCurrent) == 0 {
		return errResourceDenied
	}
	requiredBytes, err := allocatedResourceBytes(observation.allocationUnit, artifact, manifest, successorCurrent)
	if err != nil || observation.availableBytes < requiredBytes {
		return errResourceDenied
	}
	if observation.itemsKnown && observation.availableItems < resourceObjectCount {
		return errResourceDenied
	}
	return nil
}

func allocatedResourceBytes(unit uint64, artifact, manifest, successorCurrent []byte) (uint64, error) {
	parts := []uint64{uint64(len(artifact)), uint64(len(manifest)), journalV1EntryCount * journalV1Bytes, uint64(len(successorCurrent))}
	var total uint64
	for _, length := range parts {
		allocated, err := roundAllocation(length, unit)
		if err != nil || total > ^uint64(0)-allocated {
			return 0, errors.New("resource envelope overflow")
		}
		total += allocated
	}
	return total, nil
}

func roundAllocation(length, unit uint64) (uint64, error) {
	if unit == 0 || length > ^uint64(0)-(unit-1) {
		return 0, errors.New("resource allocation overflow")
	}
	return ((length + unit - 1) / unit) * unit, nil
}
