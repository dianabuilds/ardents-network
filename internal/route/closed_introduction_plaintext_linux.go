//go:build linux

package route

import (
	"encoding/binary"
	"errors"
	"time"
)

func encodeClosedIntroductionPlaintext(value ClosedIntroductionPlaintext) ([]byte, error) {
	if !validClosedIntroductionPlaintext(value) {
		return nil, errors.New("closed Introduction plaintext invalid")
	}
	raw := make([]byte, 0, closedIntroductionPlaintextSize)
	for _, field := range [][32]byte{value.Network, value.Target, value.PublicationDigest} {
		raw = append(raw, field[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, value.Revision)
	raw = append(raw, value.RendezvousNode[:]...)
	raw = binary.BigEndian.AppendUint64(raw, value.RendezvousDutyGeneration)
	for _, field := range [][32]byte{value.JoinSecret, value.HandshakeContext, value.ProfileDigest, value.ConnectionNonce} {
		raw = append(raw, field[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, value.AttachmentGeneration)
	raw = binary.BigEndian.AppendUint64(raw, uint64(value.Deadline.Unix()))
	raw = append(raw, value.InitiatorBinding[:]...)
	for _, bound := range []int64{value.WorkSafetyNotAfter, value.WorkSafetyMaximum, value.NoNewRecoveryAfter} {
		raw = binary.BigEndian.AppendUint64(raw, uint64(bound))
	}
	return raw, nil
}

func decodeClosedIntroductionPlaintext(raw []byte) (ClosedIntroductionPlaintext, error) {
	var value ClosedIntroductionPlaintext
	if len(raw) != closedIntroductionPlaintextSize {
		return value, errors.New("closed Introduction plaintext length invalid")
	}
	offset := 0
	for _, field := range []*[32]byte{&value.Network, &value.Target, &value.PublicationDigest} {
		copy(field[:], raw[offset:offset+32])
		offset += 32
	}
	value.Revision = binary.BigEndian.Uint64(raw[offset : offset+8])
	offset += 8
	copy(value.RendezvousNode[:], raw[offset:offset+32])
	offset += 32
	value.RendezvousDutyGeneration = binary.BigEndian.Uint64(raw[offset : offset+8])
	offset += 8
	for _, field := range []*[32]byte{&value.JoinSecret, &value.HandshakeContext, &value.ProfileDigest, &value.ConnectionNonce} {
		copy(field[:], raw[offset:offset+32])
		offset += 32
	}
	value.AttachmentGeneration = binary.BigEndian.Uint64(raw[offset : offset+8])
	offset += 8
	seconds := binary.BigEndian.Uint64(raw[offset : offset+8])
	offset += 8
	if seconds > 1<<63-1 {
		return value, errors.New("closed Introduction deadline overflow")
	}
	value.Deadline = time.Unix(int64(seconds), 0).UTC()
	copy(value.InitiatorBinding[:], raw[offset:offset+32])
	offset += 32
	for _, bound := range []*int64{&value.WorkSafetyNotAfter, &value.WorkSafetyMaximum, &value.NoNewRecoveryAfter} {
		seconds = binary.BigEndian.Uint64(raw[offset : offset+8])
		offset += 8
		if seconds > 1<<63-1 {
			return ClosedIntroductionPlaintext{}, errors.New("closed Introduction authority bound overflow")
		}
		*bound = int64(seconds)
	}
	if !validClosedIntroductionPlaintext(value) {
		return ClosedIntroductionPlaintext{}, errors.New("closed Introduction plaintext invalid")
	}
	return value, nil
}

func validClosedIntroductionPlaintext(value ClosedIntroductionPlaintext) bool {
	for _, field := range [][32]byte{value.Network, value.Target, value.PublicationDigest, value.RendezvousNode, value.JoinSecret,
		value.HandshakeContext, value.ProfileDigest, value.ConnectionNonce, value.InitiatorBinding} {
		if field == [32]byte{} {
			return false
		}
	}
	return value.Revision != 0 && value.RendezvousDutyGeneration != 0 && value.AttachmentGeneration != 0 && value.Deadline.Unix() > 0 &&
		value.Deadline.Equal(value.Deadline.UTC().Truncate(time.Second)) && value.Deadline.Unix() <= value.WorkSafetyNotAfter &&
		value.WorkSafetyMaximum >= value.WorkSafetyNotAfter && value.NoNewRecoveryAfter > 0 && value.NoNewRecoveryAfter <= value.WorkSafetyNotAfter
}
