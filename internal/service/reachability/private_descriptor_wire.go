package reachability

import (
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"time"
)

const privateDescriptorHeader = 2 + 5*32 + 8 + 3*32 + 8 + 8 + 2

func decodePrivateDescriptor(raw []byte) (Descriptor, error) {
	if len(raw) <= privateDescriptorHeader+ed25519.SignatureSize || len(raw) > MaximumPrivateDescriptorSize ||
		binary.BigEndian.Uint16(raw[:2]) != privateDescriptorVersion {
		return Descriptor{}, errors.New("private reachability encoding is invalid")
	}
	value := Descriptor{Version: privateDescriptorVersion}
	offset := 2
	for _, field := range []*[32]byte{&value.NetworkID, &value.Target, &value.AuthorityPublic, &value.PublicationDigest, &value.ProfileDigest} {
		copy(field[:], raw[offset:offset+32])
		offset += 32
	}
	value.Private.Revision = binary.BigEndian.Uint64(raw[offset : offset+8])
	offset += 8
	for _, field := range []*[32]byte{&value.Private.NodeID, &value.Private.Slot, &value.Private.RecipientKey} {
		copy(field[:], raw[offset:offset+32])
		offset += 32
	}
	before, after := binary.BigEndian.Uint64(raw[offset:offset+8]), binary.BigEndian.Uint64(raw[offset+8:offset+16])
	if before > 1<<63-1 || after > 1<<63-1 {
		return Descriptor{}, errors.New("private reachability time exceeds encoding")
	}
	value.Private.NotBefore, value.Private.NotAfter = time.Unix(int64(before), 0).UTC(), time.Unix(int64(after), 0).UTC()
	offset += 16
	length := int(binary.BigEndian.Uint16(raw[offset : offset+2]))
	offset += 2
	if length == 0 || offset+length+ed25519.SignatureSize != len(raw) {
		return Descriptor{}, errors.New("private reachability proof length is invalid")
	}
	value.Publication = append([]byte(nil), raw[offset:offset+length]...)
	copy(value.Signature[:], raw[offset+length:])
	return value, nil
}

func privateDescriptorTranscript(value Descriptor) []byte {
	transcript := []byte("ardents-private-reachability-v3\x00")
	for _, field := range [][32]byte{value.NetworkID, value.ProfileDigest, value.PublicationDigest} {
		transcript = append(transcript, field[:]...)
	}
	return appendPrivateRecipient(transcript, value.Private)
}

func appendPrivateRecipient(body []byte, value PrivateIntroduction) []byte {
	body = binary.BigEndian.AppendUint64(body, value.Revision)
	for _, field := range [][32]byte{value.NodeID, value.Slot, value.RecipientKey} {
		body = append(body, field[:]...)
	}
	body = binary.BigEndian.AppendUint64(body, uint64(value.NotBefore.Unix()))
	return binary.BigEndian.AppendUint64(body, uint64(value.NotAfter.Unix()))
}
