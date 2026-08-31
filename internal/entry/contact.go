package entry

import "errors"

// Contact returns the first valid State-derived adjacent candidate in fixed
// slot order. It does not select a Route, initiate a connection, or expose an
// Invite's hidden fields.
func (owner *owner) Contact() (Candidate, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closing || owner.closed {
		return Candidate{}, errors.New("entry owner is closed")
	}
	if owner.failed != nil {
		return Candidate{}, owner.failed
	}
	for slot := byte(0); slot < 2; slot++ {
		index, found := owner.state.active(slot)
		if !found {
			continue
		}
		record := &owner.state.Records[index]
		decoded, candidate, class, err := owner.validate(record.Invite)
		if err != nil {
			return Candidate{}, err
		}
		if class == Accepted && decoded.id == record.InviteID && decoded.nodeID == record.Identity && decoded.familyID == record.Family {
			return candidate, nil
		}
		retireMember(record)
		if err := owner.commit(owner.state, false); err != nil {
			owner.failed = err
			return Candidate{}, err
		}
	}
	return Candidate{}, errors.New("entry contact unavailable")
}
