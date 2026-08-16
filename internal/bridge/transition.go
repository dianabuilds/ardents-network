package bridge

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"
)

const transitionMagic = "ardents-h3-bridge-transition-v1"

// BeginContact validates one inherited owner/policy transition, durably enters
// BRIDGE, and publishes the first contact exposure before returning its bytes.
func (owner *owner) BeginContact(frame []byte, manifest [32]byte, deadline time.Time) (
	[32]byte, []byte, byte, error,
) {
	transition, err := parseTransition(frame)
	if err != nil || transition.manifest != manifest || manifest == ([32]byte{}) ||
		!owner.config.Clock().Before(deadline) {
		return [32]byte{}, nil, 0, errors.New("bridge transition is invalid")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed || owner.failed != nil || owner.state.Regime != nil || len(owner.state.Contacts) != 0 {
		return [32]byte{}, nil, 0, errors.New("bridge transition is unavailable")
	}
	index, ordinal, present := owner.firstContact()
	if !present {
		return [32]byte{}, nil, 0, errors.New("bridge contact unavailable")
	}
	record := owner.state.Records[index]
	decoded, class, err := owner.validate(record.Invite)
	if err != nil || class != classAccepted || decoded.id != record.InviteID ||
		decoded.identity != record.Identity || decoded.commitment != record.Commitment ||
		decoded.adapterProfile != record.ProfileID {
		return [32]byte{}, nil, 0, errors.New("bridge contact unavailable")
	}
	next := owner.state.clone()
	next.Regime = &regimeRecord{AttemptID: transition.attemptID, Trigger: transition.trigger,
		PolicyID: transition.policyID, Offset: transition.offset, Manifest: manifest,
		Deadline: deadline.UnixNano()}
	next.Contacts = append(next.Contacts, contactRecord{AttemptID: transition.attemptID,
		InviteID: record.InviteID, ProfileID: record.ProfileID, Slot: record.Slot,
		Ordinal: ordinal, Started: transition.offset})
	if err := owner.commit(next, false); err != nil {
		owner.failed = err
		return [32]byte{}, nil, 0, err
	}
	return decoded.identity, bytes.Clone(decoded.candidate), ordinal, nil
}

// FinishContact durably classifies one published contact and its cleanup.
func (owner *owner) FinishContact(ordinal byte, opened, cleanup bool) error {
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
		if current.Cleanup == cleanup && current.Outcome == contactOutcome(opened) {
			return nil
		}
		return errors.New("bridge contact result conflicts")
	}
	next := owner.state.clone()
	next.Contacts[index].Outcome = contactOutcome(opened)
	next.Contacts[index].Cleanup = cleanup
	if err := owner.commit(next, false); err != nil {
		owner.failed = err
		return err
	}
	return nil
}

func (owner *owner) firstContact() (int, byte, bool) {
	if index, present := owner.state.active(0); present {
		return index, 0, true
	}
	index, present := owner.state.active(1)
	return index, 2, present
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
