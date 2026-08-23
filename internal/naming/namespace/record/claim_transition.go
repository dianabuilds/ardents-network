package record

import "time"

func applyClaim(current *Record, now int64, op Op, leaseDuration, graceDuration time.Duration) (Record, error) {
	if op.Authority == "" {
		return Record{}, transitionError{Action: opClaim, Reason: "authority is required"}
	}
	if op.Generation == 0 {
		return Record{}, transitionError{Action: opClaim, Reason: "generation is required"}
	}
	if current == nil {
		if op.Generation != 1 || op.ExpectedGeneration != 0 || op.ExpectedRevision != 0 {
			return Record{}, transitionError{Action: opClaim, Reason: "first claim must create generation 1 from empty state"}
		}
	} else {
		if err := requireCurrent(current, op, opClaim); err != nil {
			return Record{}, err
		}
		if current.Lease != leaseReleased {
			return Record{}, transitionError{Action: opClaim, Reason: "name is not released"}
		}
		if op.Generation != current.Generation+1 {
			return Record{}, transitionError{Action: opClaim, Reason: "reclaim must create the next generation"}
		}
	}

	parent, err := validateParents(op.Name, op.Parents, now)
	if err != nil {
		return Record{}, transitionError{Action: opClaim, Reason: err.Error()}
	}
	leaseEnd := now + int64(leaseDuration.Seconds())
	graceEnd := leaseEnd + int64(graceDuration.Seconds())
	parentName := ""
	parentGeneration := uint64(0)
	if parent != nil {
		parentName, parentGeneration = parent.Name, parent.Generation
		leaseEnd = min(leaseEnd, parent.LeaseExpiresAt)
		graceEnd = min(graceEnd, parent.GraceExpiresAt)
	}
	if leaseEnd <= now || graceEnd < leaseEnd {
		return Record{}, transitionError{Action: opClaim, Reason: "lease does not fit inside the parent lifetime"}
	}

	continuity := uint64(1)
	if current != nil {
		continuity = current.Continuity + 1
	}
	return Record{Name: op.Name, Generation: op.Generation, Revision: 1,
		Lease: leaseActive, Consistency: consistencyCurrent, Recovery: recoveryStable,
		Authority: op.Authority, ParentName: parentName,
		ParentGeneration: parentGeneration, LeaseExpiresAt: leaseEnd,
		GraceExpiresAt: graceEnd, Continuity: continuity}, nil
}

func applyRenew(current *Record, now int64, op Op, leaseDuration, graceDuration time.Duration) (Record, error) {
	if err := requireCurrent(current, op, opRenew); err != nil {
		return Record{}, err
	}
	if current.Lease != leaseActive && current.Lease != leaseGrace {
		return Record{}, transitionError{Action: opRenew, Reason: "lease cannot be renewed"}
	}
	if (current.Lease == leaseActive && now > current.LeaseExpiresAt) ||
		(current.Lease == leaseGrace && now > current.GraceExpiresAt) {
		return Record{}, transitionError{Action: opRenew, Reason: "lease lifetime has elapsed"}
	}
	if current.Consistency != consistencyCurrent || current.Recovery != recoveryStable {
		return Record{}, transitionError{Action: opRenew, Reason: "non-current or recovering record cannot be renewed"}
	}
	if op.Authority != current.Authority {
		return Record{}, transitionError{Action: opRenew, Reason: "authority mismatch"}
	}
	parent, err := validateParents(op.Name, op.Parents, now)
	if err != nil {
		return Record{}, transitionError{Action: opRenew, Reason: err.Error()}
	}
	if !sameParent(current, parent) {
		return Record{}, transitionError{Action: opRenew, Reason: "parent generation does not match the record binding"}
	}
	leaseEnd := now + int64(leaseDuration.Seconds())
	graceEnd := leaseEnd + int64(graceDuration.Seconds())
	if parent != nil {
		leaseEnd = min(leaseEnd, parent.LeaseExpiresAt)
		graceEnd = min(graceEnd, parent.GraceExpiresAt)
	}
	if leaseEnd <= now || graceEnd < leaseEnd {
		return Record{}, transitionError{Action: opRenew, Reason: "renewal does not fit inside the parent lifetime"}
	}
	result := *current
	result.Revision++
	result.Lease = leaseActive
	result.LeaseExpiresAt, result.GraceExpiresAt = leaseEnd, graceEnd
	return result, nil
}
