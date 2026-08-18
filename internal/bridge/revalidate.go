package bridge

import "bytes"

// retireInvalidActiveLocked clears invalid active members before an attempt
// begins. The caller holds owner.mu and has not yet published an exposure.
func (owner *owner) retireInvalidActiveLocked() (bool, error) {
	next := owner.state.clone()
	changed := false
	for index := range next.Records {
		record := &next.Records[index]
		if record.Status != memberActive {
			continue
		}
		decoded, class, err := owner.validate(record.Invite)
		if err != nil {
			return false, err
		}
		if class == classAccepted && decoded.id == record.InviteID && decoded.identity == record.Identity &&
			decoded.family == record.Family && decoded.commitment == record.Commitment &&
			decoded.adapterProfile == record.ProfileID && bytes.Equal(decoded.body, signedBody(record.Invite)) {
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

// retireInvalidVerifiedLocked clears replacements that became ineligible while
// earlier derived work was still live. The caller must settle the now-terminal
// slot afterward, so a stale replacement cannot become ACTIVE.
func (owner *owner) retireInvalidVerifiedLocked(next *durableState) error {
	for index := range next.Records {
		record := &next.Records[index]
		if record.Status != memberVerified {
			continue
		}
		decoded, class, err := owner.validate(record.Invite)
		if err != nil {
			return err
		}
		if class == classAccepted && decoded.id == record.InviteID && decoded.identity == record.Identity &&
			decoded.family == record.Family && decoded.commitment == record.Commitment &&
			decoded.adapterProfile == record.ProfileID && bytes.Equal(decoded.body, signedBody(record.Invite)) {
			continue
		}
		retireMember(record)
	}
	return nil
}
