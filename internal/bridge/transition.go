package bridge

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"
)

const transitionMagic = "ardents-h3-bridge-transition-v1"
const attemptDuration = 64 * time.Second

// BeginContact validates one inherited owner/policy transition, durably enters
// BRIDGE, and publishes the first contact exposure before returning its bytes.
func (owner *owner) BeginContact(frame []byte, manifest [32]byte, deadline time.Time) (
	[32]byte, []byte, byte, uint64, time.Time, error,
) {
	transition, err := parseTransition(frame)
	now := owner.config.Clock()
	if err != nil || transition.manifest != manifest || manifest == ([32]byte{}) ||
		!deadline.IsZero() && !now.Before(deadline) {
		return [32]byte{}, nil, 0, 0, time.Time{}, errors.New("bridge-ineligible")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed || owner.failed != nil {
		return [32]byte{}, nil, 0, 0, time.Time{}, errors.New("bridge-local-denial")
	}
	if owner.state.Attempt != nil {
		if owner.state.Attempt.AttemptID != transition.attemptID {
			return [32]byte{}, nil, 0, 0, time.Time{}, errors.New("bridge-local-denial")
		}
		return [32]byte{}, nil, 0, 0, time.Time{}, errors.New(owner.attemptClass())
	}
	if owner.state.Regime != nil || len(owner.state.Contacts) != 0 {
		return [32]byte{}, nil, 0, 0, time.Time{}, errors.New("bridge-ineligible")
	}
	retired, retireErr := owner.retireInvalidActiveLocked()
	if retireErr != nil {
		return [32]byte{}, nil, 0, 0, time.Time{}, errors.New("bridge-local-denial")
	}
	index, decoded, ordinal, present := owner.firstEligibleContact()
	if !present {
		if retired {
			return [32]byte{}, nil, 0, 0, time.Time{}, errors.New("bridge-ineligible")
		}
		return [32]byte{}, nil, 0, 0, time.Time{}, errors.New("bridge-not-configured")
	}
	record := owner.state.Records[index]
	attemptDeadline := now.Add(attemptDuration)
	if !deadline.IsZero() && deadline.Before(attemptDeadline) {
		attemptDeadline = deadline
	}
	for slot := byte(0); slot < 2; slot++ {
		active, present := owner.state.active(slot)
		if !present {
			continue
		}
		member, memberClass, memberErr := owner.validate(owner.state.Records[active].Invite)
		if memberErr != nil || memberClass != classAccepted {
			continue
		}
		bound := time.Unix(member.notAfter, 0)
		if bound.Before(attemptDeadline) {
			attemptDeadline = bound
		}
	}
	if !now.Before(attemptDeadline) {
		return [32]byte{}, nil, 0, 0, time.Time{}, errors.New("bridge-ineligible")
	}
	deadlineOffset := transition.offset + uint64(attemptDeadline.Sub(now))
	next := owner.state.clone()
	next.Regime = &regimeRecord{AttemptID: transition.attemptID, Trigger: transition.trigger,
		PolicyID: transition.policyID, Offset: transition.offset, Manifest: manifest,
		Deadline: attemptDeadline.UnixNano(), DeadlineOffset: deadlineOffset}
	next.Attempt = &attemptRecord{AttemptID: transition.attemptID, Started: transition.offset,
		Deadline: attemptDeadline.UnixNano(), DeadlineOffset: deadlineOffset}
	next.Contacts = append(next.Contacts, contactRecord{AttemptID: transition.attemptID,
		InviteID: record.InviteID, ProfileID: record.ProfileID, Slot: record.Slot,
		Ordinal: ordinal, Started: transition.offset})
	if err := owner.commit(next, false); err != nil {
		owner.failed = err
		return [32]byte{}, nil, 0, 0, time.Time{}, err
	}
	return decoded.identity, bytes.Clone(decoded.candidate), ordinal, transition.offset, attemptDeadline, nil
}

// FinishContact durably classifies one published contact and its cleanup.
func (owner *owner) FinishContact(ordinal byte, terminal uint64, opened, cleanup bool) error {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	index := -1
	for candidate := range owner.state.Contacts {
		if owner.state.Contacts[candidate].Ordinal == ordinal {
			index = candidate
		}
	}
	if owner.closed || owner.failed != nil || index < 0 {
		return errors.New("bridge contact result is unavailable")
	}
	current := owner.state.Contacts[index]
	if current.Outcome != "" {
		if current.Cleanup == cleanup && current.Outcome == contactOutcome(opened) && current.Terminal == terminal {
			return nil
		}
		return errors.New("bridge contact result conflicts")
	}
	next := owner.state.clone()
	next.Contacts[index].Outcome = contactOutcome(opened)
	next.Contacts[index].Cleanup = cleanup
	next.Contacts[index].Terminal = terminal
	if terminal < current.Started {
		return errors.New("bridge contact terminal offset is invalid")
	}
	if opened {
		next.Attempt.Terminal, next.Attempt.TerminalOffset = "opened", terminal
	} else if !cleanup {
		next.Attempt.Terminal, next.Attempt.TerminalOffset = "bridge-local-denial", terminal
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

func (owner *owner) attemptClass() string {
	if owner.state.Attempt != nil && owner.state.Attempt.Terminal != "" {
		if owner.state.Attempt.Terminal == "opened" {
			return "bridge-attempt-exhausted"
		}
		return owner.state.Attempt.Terminal
	}
	return "bridge-local-denial"
}

func (owner *owner) firstEligibleContact() (int, invite, byte, bool) {
	for slot := byte(0); slot < 2; slot++ {
		index, present := owner.state.active(slot)
		if !present {
			continue
		}
		record := owner.state.Records[index]
		decoded, class, err := owner.validate(record.Invite)
		if err == nil && class == classAccepted && decoded.id == record.InviteID &&
			decoded.identity == record.Identity && decoded.commitment == record.Commitment &&
			decoded.adapterProfile == record.ProfileID {
			return index, decoded, slot * 2, true
		}
	}
	return 0, invite{}, 0, false
}

type transitionFrame struct {
	attemptID [32]byte
	trigger   string
	policyID  [32]byte
	offset    uint64
	manifest  [32]byte
}

func parseTransition(raw []byte) (transitionFrame, error) {
	want := len(transitionMagic) + 32 + 1 + 32 + 8 + 32
	if len(raw) != want || string(raw[:len(transitionMagic)]) != transitionMagic {
		return transitionFrame{}, errors.New("bridge transition frame is invalid")
	}
	reader := raw[len(transitionMagic):]
	var frame transitionFrame
	copy(frame.attemptID[:], reader[:32])
	trigger := reader[32]
	copy(frame.policyID[:], reader[33:65])
	frame.offset = binary.BigEndian.Uint64(reader[65:73])
	copy(frame.manifest[:], reader[73:])
	if trigger == 1 {
		frame.trigger = "owner"
	} else if trigger == 2 {
		frame.trigger = "policy"
	}
	if frame.attemptID == ([32]byte{}) || frame.policyID == ([32]byte{}) || frame.manifest == ([32]byte{}) ||
		frame.trigger == "" || frame.offset == 0 {
		return transitionFrame{}, errors.New("bridge transition frame is invalid")
	}
	return frame, nil
}

func contactOutcome(opened bool) string {
	if opened {
		return "opened"
	}
	return "failed"
}
