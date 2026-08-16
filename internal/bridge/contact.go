package bridge

import (
	"bytes"
	"errors"
)

// Contact returns the first currently valid member in fixed slot order and a
// copy of its opaque candidate envelope. It performs no process or network
// action and revalidates authenticated and local-role facts on every call.
func (owner *owner) Contact() ([32]byte, []byte, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed {
		return [32]byte{}, nil, errors.New("bridge owner is closed")
	}
	if owner.failed != nil {
		return [32]byte{}, nil, owner.failed
	}
	for slot := byte(0); slot < 2; slot++ {
		index, present := owner.state.active(slot)
		if !present {
			continue
		}
		record := &owner.state.Records[index]
		decoded, class, err := owner.validate(record.Invite)
		if err != nil {
			return [32]byte{}, nil, err
		}
		if class == classAccepted && decoded.id == record.InviteID && decoded.identity == record.Identity &&
			decoded.commitment == record.Commitment && decoded.adapterProfile == record.ProfileID &&
			bytes.Equal(decoded.body, signedBody(record.Invite)) {
			return decoded.identity, bytes.Clone(decoded.candidate), nil
		}
		record.Status = memberRetired
		record.Invite = nil
		record.Commitment = [32]byte{}
		record.ProfileID = ""
		if err := owner.commit(owner.state, false); err != nil {
			owner.failed = err
			return [32]byte{}, nil, err
		}
	}
	return [32]byte{}, nil, errors.New("bridge contact unavailable")
}

func signedBody(raw []byte) []byte {
	reader := binaryDecoder{raw: raw}
	if string(reader.take(len(inviteMagic))) != inviteMagic {
		return nil
	}
	return reader.take(int(reader.uint16()))
}
