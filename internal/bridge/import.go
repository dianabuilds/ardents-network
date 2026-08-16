package bridge

import (
	"errors"
)

// Import validates and atomically publishes one bounded Invite. Classified
// input rejection is returned in Result; error is reserved for owner/store
// failures.
func (owner *owner) Import(raw []byte) (Result, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed {
		return Result{}, errors.New("bridge owner is closed")
	}
	if owner.failed != nil {
		return Result{}, errors.New("bridge owner requires reopen after a failed commit")
	}
	decoded, class, err := owner.validate(raw)
	if err != nil {
		return Result{}, err
	}
	result := Result{Class: class, InviteID: decoded.id, Slot: decoded.slot, Generation: decoded.slotGeneration}
	if existing, found := owner.state.find(decoded.id); found {
		if existing.Status != memberActive {
			result.Class = classReplay
		} else if class == classAccepted {
			result.Class = classAlreadyPresent
		}
		return result, nil
	}
	if class != classAccepted {
		return result, nil
	}
	if decoded.slot > 1 || decoded.slotGeneration < 1 || decoded.slotGeneration > 2 {
		result.Class = classInvalid
		return result, nil
	}
	next := owner.state.clone()
	activeIndex, occupied := next.active(decoded.slot)
	replacesActive := occupied
	if !occupied {
		if decoded.slotGeneration != 1 || decoded.replaces != nil || slotWasUsed(next, decoded.slot) {
			result.Class = classReplacementRejected
			return result, nil
		}
	} else {
		active := next.Records[activeIndex]
		if decoded.slotGeneration != 2 || decoded.replaces == nil || *decoded.replaces != active.InviteID ||
			active.Generation != 1 {
			result.Class = classReplacementRejected
			return result, nil
		}
		if next.Attempt != nil && next.Attempt.Terminal == "" {
			next.Records[activeIndex].Status = memberDraining
		} else {
			next.Records[activeIndex].Status = memberRetired
			next.Records[activeIndex].Invite = nil
			next.Records[activeIndex].Commitment = [32]byte{}
			next.Records[activeIndex].ProfileID = ""
		}
	}
	if len(next.Records) >= 4 {
		result.Class = classSetFull
		return result, nil
	}
	next.Records = append(next.Records, memberRecord{
		InviteID: decoded.id, Identity: decoded.identity, Family: decoded.family,
		Commitment: decoded.commitment, ProfileID: decoded.adapterProfile,
		Slot: decoded.slot, Generation: decoded.slotGeneration,
		Status: memberActive, Invite: append([]byte(nil), raw...),
	})
	if next.Attempt != nil && next.Attempt.Terminal == "" && replacesActive {
		next.Records[len(next.Records)-1].Status = memberVerified
	}
	if err := owner.commit(next, !replacesActive); err != nil {
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
