package namespace

func applyPublish(current *Record, now int64, op Op) (Record, error) {
	if err := requireCurrent(current, op, opPublish); err != nil {
		return Record{}, err
	}
	if op.Authority != current.Authority {
		return Record{}, transitionError{Action: opPublish, Reason: "authority mismatch"}
	}
	if op.Target == [32]byte{} {
		return Record{}, transitionError{Action: opPublish, Reason: "Service Target is required"}
	}
	if current.Target == op.Target {
		return Record{}, transitionError{Action: opPublish, Reason: "Service Target is unchanged"}
	}
	if op.RecordNotAfter <= now*1_000 {
		return Record{}, transitionError{Action: opPublish, Reason: "Record validity has elapsed"}
	}
	if ok, reason := liveLease(*current, now); !ok {
		return Record{}, transitionError{Action: opPublish, Reason: reason}
	}
	parent, err := validateParents(op.Name, op.Parents, now)
	if err != nil || !sameParent(current, parent) {
		return Record{}, transitionError{Action: opPublish, Reason: "parent lineage is missing or stale"}
	}
	if op.RecordNotAfter > lineageNotAfter(*current, op.Parents)*1_000 {
		return Record{}, transitionError{Action: opPublish, Reason: "Record validity exceeds Lease lineage"}
	}
	result := *current
	result.Revision++
	result.Target, result.RecordNotAfter = op.Target, op.RecordNotAfter
	return result, nil
}

func lineageNotAfter(record Record, parents []Record) int64 {
	bound := leaseNotAfter(record)
	for _, parent := range parents {
		if leaseNotAfter(parent) < bound {
			bound = leaseNotAfter(parent)
		}
	}
	return bound
}

func leaseNotAfter(record Record) int64 {
	if record.Lease == leaseGrace {
		return record.GraceExpiresAt
	}
	return record.LeaseExpiresAt
}
