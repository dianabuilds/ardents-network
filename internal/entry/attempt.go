package entry

import (
	"context"
	"errors"
	"net"
	"time"
)

const maximumContacts = 4

// Acquire opens one bounded sequence of State-derived adjacent candidates.
// It persists each contact before exposure and does not retry after an opener
// reports unclean failure. The caller supplies the carrier implementation;
// Entry supplies the exact opaque Invite but no endpoint discovery or Route
// selection.
func (owner *owner) Acquire(ctx context.Context, attempt Attempt, open CandidateOpener) (net.Conn, func() error, error) {
	if open == nil {
		return nil, nil, errors.New("entry opener is unavailable")
	}
	record, candidate, ordinal, deadline, err := owner.beginAttempt(attempt)
	if err != nil {
		return nil, nil, err
	}
	for {
		contactCtx, cancel, contextErr := boundedContactContext(ctx, deadline)
		if contextErr != nil {
			finishErr := owner.finishContact(ordinal, false, true)
			terminalErr := owner.terminalize("entry-deadline-exceeded")
			return nil, nil, errors.Join(contextErr, finishErr, terminalErr)
		}
		presentation := Presentation{InviteID: record.InviteID, Invite: append([]byte(nil), record.Invite...)}
		connection, cleanup, cleanupComplete, openErr := open(contactCtx, candidate, presentation, deadline)
		cancel()
		if openErr == nil && (connection == nil || cleanup == nil) {
			openErr = errors.New("entry opener returned an incomplete result")
			cleanupComplete = false
		}
		if openErr != nil {
			finishErr := owner.finishContact(ordinal, false, cleanupComplete)
			if !cleanupComplete || finishErr != nil {
				return nil, nil, errors.Join(errors.New("entry local denial"), openErr, finishErr)
			}
			record, candidate, ordinal, deadline, err = owner.nextContact()
			if err != nil {
				return nil, nil, errors.Join(openErr, err)
			}
			continue
		}
		guarded := &guardedConnection{Conn: connection, deadline: deadline, clock: owner.config.Clock, confident: owner.config.TimeConfident}
		if err := guarded.SetDeadline(deadline); err != nil {
			cleanupErr := normalizeCleanup(cleanup())
			finishErr := owner.finishContact(ordinal, false, cleanupErr == nil)
			return nil, nil, errors.Join(errors.New("entry local denial"), err, cleanupErr, finishErr)
		}
		return guarded, func() error {
			cleanupErr := normalizeCleanup(cleanup())
			finishErr := owner.finishContact(ordinal, true, cleanupErr == nil)
			if cleanupErr != nil {
				return errors.Join(errors.New("entry local denial"), cleanupErr, finishErr)
			}
			return finishErr
		}, nil
	}
}

func (owner *owner) terminalize(class string) error {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed || owner.failed != nil || owner.state.Attempt == nil {
		return errors.New("entry attempt is unavailable")
	}
	if owner.state.Attempt.Terminal != "" {
		return nil
	}
	return owner.endAttemptLocked(class, owner.config.Clock())
}

func (owner *owner) beginAttempt(input Attempt) (memberRecord, Candidate, byte, time.Time, error) {
	if input.ID == [32]byte{} || input.Deadline.IsZero() {
		return memberRecord{}, Candidate{}, 0, time.Time{}, errors.New("entry attempt is invalid")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed || owner.failed != nil {
		return memberRecord{}, Candidate{}, 0, time.Time{}, errors.New("entry local denial")
	}
	now := owner.config.Clock().UTC()
	deadline := input.Deadline.UTC()
	if !owner.config.TimeConfident() || !now.Before(deadline) ||
		owner.state.Attempt != nil && (!reusableEntryAttempt(owner.state) || owner.state.Attempt.ID == input.ID) {
		return memberRecord{}, Candidate{}, 0, time.Time{}, errors.New("entry attempt is unavailable")
	}
	if _, err := owner.retireInvalidActiveLocked(); err != nil {
		return memberRecord{}, Candidate{}, 0, time.Time{}, err
	}
	record, candidate, ordinal, recordDeadline, found := owner.firstEligibleContactLocked()
	if !found {
		return memberRecord{}, Candidate{}, 0, time.Time{}, errors.New("entry contact unavailable")
	}
	if recordDeadline.Before(deadline) {
		deadline = recordDeadline
	}
	if !now.Before(deadline) {
		return memberRecord{}, Candidate{}, 0, time.Time{}, errors.New("entry attempt is unavailable")
	}
	next := owner.state.clone()
	// A fully cleaned opened attachment is a completed operation, not a retry
	// authority. Retain it until the next distinct attempt is durably started,
	// then replace the bounded terminal journal with the new live journal.
	if next.Attempt != nil {
		next.Attempt, next.Contacts = nil, nil
	}
	next.Attempt = &attemptRecord{ID: input.ID, Started: now.UnixNano(), Deadline: deadline.UnixNano()}
	next.Contacts = append(next.Contacts, contactRecord{AttemptID: input.ID, InviteID: record.InviteID, Slot: record.Slot, Ordinal: ordinal, Started: now.UnixNano()})
	if err := owner.commit(next, false); err != nil {
		owner.failed = err
		return memberRecord{}, Candidate{}, 0, time.Time{}, err
	}
	return record, candidate, ordinal, deadline, nil
}

func reusableEntryAttempt(state durableState) bool {
	if state.Attempt == nil || state.Attempt.Terminal != "opened" || len(state.Contacts) == 0 {
		return false
	}
	opened := false
	for _, contact := range state.Contacts {
		if contact.Outcome == "" || contact.Terminal == 0 || !contact.Cleanup {
			return false
		}
		if contact.Outcome == "opened" {
			if opened {
				return false
			}
			opened = true
		}
	}
	return opened
}

func (owner *owner) nextContact() (memberRecord, Candidate, byte, time.Time, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed || owner.failed != nil || owner.state.Attempt == nil || owner.state.Attempt.Terminal != "" || len(owner.state.Contacts) == 0 {
		return memberRecord{}, Candidate{}, 0, time.Time{}, errors.New("entry attempt is unavailable")
	}
	last := owner.state.Contacts[len(owner.state.Contacts)-1]
	if last.Outcome != "failed" || !last.Cleanup {
		return memberRecord{}, Candidate{}, 0, time.Time{}, errors.New("entry local denial")
	}
	now := owner.config.Clock().UTC()
	deadline := time.Unix(0, owner.state.Attempt.Deadline).UTC()
	if !owner.config.TimeConfident() || !now.Before(deadline) {
		return memberRecord{}, Candidate{}, 0, time.Time{}, owner.endAttemptLocked("entry-deadline-exceeded", now)
	}
	for ordinal := last.Ordinal + 1; ordinal < maximumContacts; ordinal++ {
		record, candidate, recordDeadline, found := owner.recordForOrdinalLocked(ordinal)
		if !found {
			continue
		}
		if recordDeadline.Before(deadline) {
			deadline = recordDeadline
		}
		if !now.Before(deadline) {
			return memberRecord{}, Candidate{}, 0, time.Time{}, owner.endAttemptLocked("entry-deadline-exceeded", now)
		}
		next := owner.state.clone()
		next.Contacts = append(next.Contacts, contactRecord{AttemptID: next.Attempt.ID, InviteID: record.InviteID, Slot: record.Slot, Ordinal: ordinal, Started: now.UnixNano()})
		if err := owner.commit(next, false); err != nil {
			owner.failed = err
			return memberRecord{}, Candidate{}, 0, time.Time{}, err
		}
		return record, candidate, ordinal, deadline, nil
	}
	return memberRecord{}, Candidate{}, 0, time.Time{}, owner.endAttemptLocked("entry-attempt-exhausted", now)
}

func (owner *owner) finishContact(ordinal byte, opened, cleanup bool) error {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed || owner.failed != nil || owner.state.Attempt == nil {
		return errors.New("entry contact result is unavailable")
	}
	index := -1
	for candidate := range owner.state.Contacts {
		if owner.state.Contacts[candidate].Ordinal == ordinal {
			index = candidate
		}
	}
	if index < 0 {
		return errors.New("entry contact result is unavailable")
	}
	current := owner.state.Contacts[index]
	if current.Outcome != "" {
		if current.Outcome == contactOutcome(opened) && current.Cleanup == cleanup {
			return nil
		}
		return errors.New("entry contact result conflicts")
	}
	now := owner.config.Clock().UTC()
	next := owner.state.clone()
	next.Contacts[index].Outcome = contactOutcome(opened)
	next.Contacts[index].Cleanup = cleanup
	next.Contacts[index].Terminal = now.UnixNano()
	if opened {
		next.Attempt.Terminal, next.Attempt.Ended = "opened", now.UnixNano()
	} else if !cleanup {
		next.Attempt.Terminal, next.Attempt.Ended = "entry-local-denial", now.UnixNano()
	}
	if next.Attempt.Terminal != "" {
		if err := owner.retireInvalidVerifiedLocked(&next); err != nil {
			return err
		}
		next.settleReplacements()
	}
	if err := owner.commit(next, false); err != nil {
		owner.failed = err
		return err
	}
	return nil
}

func (owner *owner) endAttemptLocked(class string, ended time.Time) error {
	if owner.state.Attempt == nil || owner.state.Attempt.Terminal != "" {
		return errors.New("entry attempt is unavailable")
	}
	next := owner.state.clone()
	next.Attempt.Terminal, next.Attempt.Ended = class, ended.UTC().UnixNano()
	if err := owner.retireInvalidVerifiedLocked(&next); err != nil {
		return err
	}
	next.settleReplacements()
	if err := owner.commit(next, false); err != nil {
		owner.failed = err
		return err
	}
	return errors.New(class)
}

func (owner *owner) firstEligibleContactLocked() (memberRecord, Candidate, byte, time.Time, bool) {
	for ordinal := byte(0); ordinal < maximumContacts; ordinal++ {
		if record, candidate, deadline, found := owner.recordForOrdinalLocked(ordinal); found {
			return record, candidate, ordinal, deadline, true
		}
	}
	return memberRecord{}, Candidate{}, 0, time.Time{}, false
}

func (owner *owner) recordForOrdinalLocked(ordinal byte) (memberRecord, Candidate, time.Time, bool) {
	slot := ordinal / 2
	if ordinal%2 == 1 {
		for _, contact := range owner.state.Contacts {
			if contact.Ordinal == ordinal-1 {
				record, found := owner.state.find(contact.InviteID)
				if !found || record.Status != memberActive {
					return memberRecord{}, Candidate{}, time.Time{}, false
				}
				return owner.validRecord(record)
			}
		}
		return memberRecord{}, Candidate{}, time.Time{}, false
	}
	index, found := owner.state.active(slot)
	if !found {
		return memberRecord{}, Candidate{}, time.Time{}, false
	}
	return owner.validRecord(owner.state.Records[index])
}

func (owner *owner) validRecord(record memberRecord) (memberRecord, Candidate, time.Time, bool) {
	decoded, candidate, class, err := owner.validate(record.Invite)
	if err != nil || class != Accepted || decoded.id != record.InviteID || decoded.nodeID != record.Identity || decoded.familyID != record.Family {
		return memberRecord{}, Candidate{}, time.Time{}, false
	}
	return record, candidate, time.Unix(decoded.notAfter, 0).UTC(), true
}

func boundedContactContext(parent context.Context, deadline time.Time) (context.Context, context.CancelFunc, error) {
	if !time.Now().Before(deadline) {
		return nil, nil, errors.New("entry deadline exceeded")
	}
	if parentDeadline, found := parent.Deadline(); found && parentDeadline.Before(deadline) {
		deadline = parentDeadline
	}
	if !time.Now().Before(deadline) {
		return nil, nil, errors.New("entry deadline exceeded")
	}
	ctx, cancel := context.WithDeadline(parent, deadline)
	return ctx, cancel, nil
}

func normalizeCleanup(err error) error {
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func contactOutcome(opened bool) string {
	if opened {
		return "opened"
	}
	return "failed"
}
