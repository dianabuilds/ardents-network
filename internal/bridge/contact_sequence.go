package bridge

import (
	"bytes"
	"context"
	"errors"
	"time"
)

const minimumContactSpacing = uint64(time.Second)

// NextContact durably publishes the next eligible fixed ordinal after one
// failed contact and verified cleanup.
func (owner *owner) NextContact(ctx context.Context) ([32]byte, []byte, byte, error) {
	owner.mu.Lock()
	if owner.closed || owner.failed != nil || owner.state.Regime == nil || len(owner.state.Contacts) == 0 {
		owner.mu.Unlock()
		return [32]byte{}, nil, 0, errors.New("bridge-local-denial")
	}
	if owner.state.Attempt == nil || owner.state.Attempt.Terminal != "" {
		class := owner.attemptClass()
		owner.mu.Unlock()
		return [32]byte{}, nil, 0, errors.New(class)
	}
	last := owner.state.Contacts[len(owner.state.Contacts)-1]
	deadline := time.Unix(0, owner.state.Regime.Deadline)
	count := len(owner.state.Contacts)
	if last.Outcome != "failed" || !last.Cleanup {
		owner.mu.Unlock()
		return [32]byte{}, nil, 0, errors.New("bridge-local-denial")
	}
	if _, _, _, terminal, present := owner.nextEligible(last.Ordinal); !present {
		err := owner.endAttemptLocked(terminal, last.Terminal)
		owner.mu.Unlock()
		return [32]byte{}, nil, 0, err
	}
	if now := owner.config.Clock(); !now.Before(deadline) || deadline.Sub(now) <= time.Second {
		err := owner.endAttemptLocked("bridge-deadline-exceeded", last.Terminal)
		owner.mu.Unlock()
		return [32]byte{}, nil, 0, err
	}
	owner.mu.Unlock()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		owner.mu.Lock()
		err := owner.endAttemptLocked("bridge-deadline-exceeded", last.Terminal)
		owner.mu.Unlock()
		return [32]byte{}, nil, 0, errors.Join(err, ctx.Err())
	case <-timer.C:
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed || owner.failed != nil || len(owner.state.Contacts) != count ||
		owner.state.Contacts[count-1] != last {
		return [32]byte{}, nil, 0, errors.New("bridge-local-denial")
	}
	if !owner.config.Clock().Before(deadline) {
		return [32]byte{}, nil, 0, owner.endAttemptLocked("bridge-deadline-exceeded", last.Terminal)
	}
	record, decoded, ordinal, terminal, present := owner.nextEligible(last.Ordinal)
	if !present {
		return [32]byte{}, nil, 0, owner.endAttemptLocked(terminal, last.Terminal)
	}
	next := owner.state.clone()
	next.Contacts = append(next.Contacts, contactRecord{AttemptID: next.Regime.AttemptID,
		InviteID: record.InviteID, ProfileID: record.ProfileID, Slot: record.Slot,
		Ordinal: ordinal, Started: last.Terminal + minimumContactSpacing})
	if err := owner.commit(next, false); err != nil {
		owner.failed = err
		return [32]byte{}, nil, 0, errors.Join(errors.New("bridge-local-denial"), err)
	}
	return decoded.identity, bytes.Clone(decoded.candidate), ordinal, nil
}

func (owner *owner) nextEligible(after byte) (memberRecord, invite, byte, string, bool) {
	terminal := "bridge-attempt-exhausted"
	for ordinal := after + 1; ordinal < 4; ordinal++ {
		record, present := owner.recordForOrdinal(ordinal)
		if !present {
			continue
		}
		decoded, class, err := owner.validate(record.Invite)
		if err == nil && class == classAccepted && decoded.id == record.InviteID &&
			decoded.identity == record.Identity && decoded.commitment == record.Commitment &&
			decoded.adapterProfile == record.ProfileID {
			return record, decoded, ordinal, "", true
		}
		terminal = "bridge-ineligible"
	}
	return memberRecord{}, invite{}, 0, terminal, false
}

func (owner *owner) endAttemptLocked(class string, offset uint64) error {
	if owner.state.Attempt == nil {
		return errors.New("bridge-local-denial")
	}
	if owner.state.Attempt.Terminal != "" {
		return errors.New(owner.state.Attempt.Terminal)
	}
	next := owner.state.clone()
	if offset < next.Attempt.Started {
		offset = next.Attempt.Started
	}
	next.Attempt.Terminal, next.Attempt.TerminalOffset = class, offset
	if err := owner.retireInvalidVerifiedLocked(&next); err != nil {
		return errors.Join(errors.New("bridge-local-denial"), err)
	}
	next.settleReplacements()
	if err := owner.commit(next, false); err != nil {
		owner.failed = err
		return errors.Join(errors.New("bridge-local-denial"), err)
	}
	return errors.New(class)
}

func (owner *owner) recordForOrdinal(ordinal byte) (memberRecord, bool) {
	slot := ordinal / 2
	if ordinal%2 != 0 {
		for _, contact := range owner.state.Contacts {
			if contact.Ordinal != ordinal-1 {
				continue
			}
			if record, found := owner.state.find(contact.InviteID); found &&
				record.Status == memberActive {
				return record, true
			}
		}
		return memberRecord{}, false
	}
	index, present := owner.state.active(slot)
	if !present {
		return memberRecord{}, false
	}
	record := owner.state.Records[index]
	return record, true
}
