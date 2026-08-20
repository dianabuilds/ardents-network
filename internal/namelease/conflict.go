package namelease

func applyConflict(current *Record, op Op) (Record, error) {
	if err := requireCurrent(current, op, opConflict); err != nil {
		return Record{}, err
	}
	if current.Lease == leaseReleased {
		return Record{}, transitionError{Action: opConflict, Reason: "released names cannot enter conflict"}
	}
	if current.Consistency != consistencyCurrent {
		return Record{}, transitionError{Action: opConflict, Reason: "consistency is already non-current"}
	}
	if op.ConflictContext == "" {
		return Record{}, transitionError{Action: opConflict, Reason: "conflict context is required"}
	}
	result := *current
	result.Revision++
	result.Consistency = consistencyConflict
	result.ConflictIdentifier = op.ConflictContext
	return result, nil
}
