package namelease

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
	if ok, reason := liveLease(*current, now); !ok {
		return Record{}, transitionError{Action: opPublish, Reason: reason}
	}
	parent, err := validateParents(op.Name, op.Parents, now)
	if err != nil || !sameParent(current, parent) {
		return Record{}, transitionError{Action: opPublish, Reason: "parent lineage is missing or stale"}
	}
	result := *current
	result.Revision++
	result.Target = op.Target
	return result, nil
}
