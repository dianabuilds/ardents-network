package record

func applyRelease(current *Record, op Op) (Record, error) {
	if err := requireCurrent(current, op, opRelease); err != nil {
		return Record{}, err
	}
	if op.Authority != current.Authority {
		return Record{}, transitionError{Action: opRelease, Reason: "authority mismatch"}
	}
	if current.Lease == leaseReleased {
		return Record{}, transitionError{Action: opRelease, Reason: "name is already released"}
	}
	if current.Consistency != consistencyCurrent {
		return Record{}, transitionError{Action: opRelease, Reason: "non-current consistency cannot force release"}
	}
	if current.Recovery != recoveryStable {
		return Record{}, transitionError{Action: opRelease, Reason: "recovery-pending record cannot be released"}
	}
	result := *current
	result.Revision++
	result.Lease = leaseReleased
	result.LeaseExpiresAt, result.GraceExpiresAt = 0, 0
	return result, nil
}

func applyAdvance(current *Record, seconds int64, op Op) (Record, error) {
	if err := requireCurrent(current, op, opAdvance); err != nil {
		return Record{}, err
	}
	result := *current
	if current.Recovery == recoveryPending {
		return Record{}, transitionError{Action: opAdvance, Reason: "pending recovery requires threshold completion"}
	}
	switch current.Lease {
	case leaseActive:
		if seconds <= current.LeaseExpiresAt {
			return result, nil
		}
		result.Revision++
		if current.GraceExpiresAt > 0 && seconds <= current.GraceExpiresAt {
			result.Lease = leaseGrace
			return result, nil
		}
		result.Lease = leaseReleased
		result.LeaseExpiresAt, result.GraceExpiresAt = 0, 0
		return result, nil
	case leaseGrace:
		if seconds <= current.GraceExpiresAt {
			return result, nil
		}
		result.Revision++
		result.Lease = leaseReleased
		result.LeaseExpiresAt, result.GraceExpiresAt = 0, 0
		return result, nil
	case leaseReleased:
		return result, nil
	default:
		return Record{}, transitionError{Action: opAdvance, Reason: "invalid Lease state"}
	}
}
