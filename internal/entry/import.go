package entry

import "errors"

// Import validates and atomically publishes one bounded Entry Invite. Input
// rejection is classified; errors are reserved for owner and durable-store
// failures.
func (owner *owner) Import(raw []byte) (Result, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed {
		return Result{}, errors.New("entry owner is closed")
	}
	if owner.failed != nil {
		return Result{}, errors.New("entry owner requires reopen after a failed commit")
	}
	decoded, _, class, err := owner.validate(raw)
	if err != nil {
		return Result{}, err
	}
	result := Result{Class: class, InviteID: decoded.id, Slot: decoded.slot, Generation: decoded.slotGeneration}
	if existing, found := owner.state.find(decoded.id); found {
		if existing.Status != memberActive {
			result.Class = Replay
		} else if class == Accepted {
			result.Class = AlreadyPresent
		}
		return result, nil
	}
	if class != Accepted {
		return result, nil
	}
	next := owner.state.clone()
	if len(next.Records) >= 4 {
		result.Class = SetFull
		return result, nil
	}
	activeIndex, occupied := next.active(decoded.slot)
	if !occupied {
		if decoded.slotGeneration != 1 || decoded.replaces != nil || slotWasUsed(next, decoded.slot) {
			result.Class = ReplacementRejected
			return result, nil
		}
	} else {
		active := next.Records[activeIndex]
		if decoded.slotGeneration != 2 || decoded.replaces == nil || *decoded.replaces != active.InviteID || active.Generation != 1 {
			result.Class = ReplacementRejected
			return result, nil
		}
		retireMember(&next.Records[activeIndex])
	}
	next.Records = append(next.Records, memberRecord{InviteID: decoded.id, Identity: decoded.nodeID, Family: decoded.familyID,
		Slot: decoded.slot, Generation: decoded.slotGeneration, Status: memberActive, Invite: append([]byte(nil), raw...)})
	if err := owner.commit(next, true); err != nil {
		owner.failed = err
		return Result{}, err
	}
	return result, nil
}

func slotWasUsed(state durableState, slot byte) bool {
	for _, record := range state.Records {
		if record.Slot == slot {
			return true
		}
	}
	return false
}
