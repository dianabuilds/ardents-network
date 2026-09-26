package entry

import "time"

// validRecord projects one retained member record against current State. The
// Invite must still decode to an Accepted candidate whose pinned identity
// facts exactly match the record.
func (owner *owner) validRecord(record memberRecord) (memberRecord, Candidate, time.Time, bool) {
	decoded, candidate, class, err := owner.validate(record.Invite)
	if err != nil || class != Accepted || decoded.id != record.InviteID || decoded.nodeID != record.Identity || decoded.familyID != record.Family {
		return memberRecord{}, Candidate{}, time.Time{}, false
	}
	return record, candidate, time.Unix(decoded.notAfter, 0).UTC(), true
}

// retireInvalidVerifiedLocked ensures a replacement cannot become active after
// a terminal legacy attempt if its State authority disappeared meanwhile.
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
