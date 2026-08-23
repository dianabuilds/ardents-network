package entry

// retireInvalidActiveLocked clears stale active Invites before a carrier is
// exposed. The caller holds owner.mu.
func (owner *owner) retireInvalidActiveLocked() (bool, error) {
	next := owner.state.clone()
	changed := false
	for index := range next.Records {
		record := &next.Records[index]
		if record.Status != memberActive {
			continue
		}
		if _, _, _, found := owner.validRecord(*record); found {
			continue
		}
		retireMember(record)
		changed = true
	}
	if !changed {
		return false, nil
	}
	if err := owner.commit(next, false); err != nil {
		owner.failed = err
		return false, err
	}
	return true, nil
}

// retireInvalidVerifiedLocked ensures a replacement cannot become active after
// a live attempt if its State authority disappeared meanwhile.
func (owner *owner) retireInvalidVerifiedLocked(next *durableState) error {
	for index := range next.Records {
		record := &next.Records[index]
		if record.Status != memberVerified {
			continue
		}
		if _, _, _, found := owner.validRecord(*record); found {
			continue
		}
		retireMember(record)
	}
	return nil
}
