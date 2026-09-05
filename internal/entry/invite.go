package entry

import (
	"bytes"
	"crypto/sha256"
)

const (
	inviteMagic       = "ardents-entry-invite-v2"
	inviteSignature   = "ardents-entry-invite-signature-v2"
	inviteIdentifier  = "ardents-entry-invite-id-v2"
	inviteWireVersion = uint16(2)
	maximumInviteSize = 1024
)

type invite struct {
	body                                    []byte
	signature                               []byte
	id, networkID, epochDigest, issuerID    [32]byte
	nodeID, familyID                        [32]byte
	recordDigest, domainProofDigest         [32]byte
	epoch                                   uint64
	profile                                 string
	assignmentNotAfter, notBefore, notAfter int64
	slotGeneration, slot                    byte
	replaces                                *[32]byte
}

func decodeInvite(raw []byte) (invite, Class) {
	if len(raw) < len(inviteMagic)+2+64 || len(raw) > maximumInviteSize {
		return invite{}, Invalid
	}
	decoder := byteReader{raw: raw}
	if !bytes.Equal(decoder.take(len(inviteMagic)), []byte(inviteMagic)) {
		return invite{}, Invalid
	}
	body := decoder.take(int(decoder.uint16()))
	signature := decoder.take(64)
	if decoder.failed || !decoder.done() {
		return invite{}, Invalid
	}
	bodyReader := byteReader{raw: body}
	if bodyReader.uint16() != inviteWireVersion {
		if bodyReader.failed {
			return invite{}, Invalid
		}
		return invite{}, Incompatible
	}
	decoded := invite{body: bytes.Clone(body), signature: bytes.Clone(signature)}
	copy(decoded.networkID[:], bodyReader.take(32))
	decoded.epoch = bodyReader.uint64()
	copy(decoded.epochDigest[:], bodyReader.take(32))
	decoded.profile = string(bodyReader.short(63))
	copy(decoded.issuerID[:], bodyReader.take(32))
	copy(decoded.nodeID[:], bodyReader.take(32))
	copy(decoded.familyID[:], bodyReader.take(32))
	copy(decoded.recordDigest[:], bodyReader.take(32))
	copy(decoded.domainProofDigest[:], bodyReader.take(32))
	decoded.assignmentNotAfter = bodyReader.int64()
	decoded.notBefore = bodyReader.int64()
	decoded.notAfter = bodyReader.int64()
	decoded.slotGeneration = bodyReader.byte()
	decoded.slot = bodyReader.byte()
	if bodyReader.byte() == 1 {
		decoded.replaces = new([32]byte)
		copy(decoded.replaces[:], bodyReader.take(32))
	} else if bodyReader.last != 0 {
		bodyReader.failed = true
	}
	if bodyReader.failed || !bodyReader.done() || !validProfile(decoded.profile) || decoded.networkID == [32]byte{} ||
		decoded.epoch == 0 || decoded.epochDigest == [32]byte{} || decoded.issuerID == [32]byte{} ||
		decoded.nodeID == [32]byte{} || decoded.familyID == [32]byte{} || decoded.recordDigest == [32]byte{} ||
		decoded.domainProofDigest == [32]byte{} || decoded.assignmentNotAfter <= 0 || decoded.notBefore <= 0 ||
		decoded.notAfter <= decoded.notBefore || decoded.slot > 1 || decoded.slotGeneration < 1 || decoded.slotGeneration > 2 ||
		decoded.slotGeneration == 1 && decoded.replaces != nil || decoded.slotGeneration == 2 && decoded.replaces == nil {
		return invite{}, Invalid
	}
	identifier := sha256.New()
	identifier.Write([]byte(inviteIdentifier))
	identifier.Write([]byte{0})
	identifier.Write(decoded.body)
	copy(decoded.id[:], identifier.Sum(nil))
	return decoded, Accepted
}

func validProfile(value string) bool {
	if value != profileID || len(value) > 63 {
		return false
	}
	for _, character := range []byte(value) {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

type byteReader struct {
	raw    []byte
	failed bool
	last   byte
}

func (reader *byteReader) take(length int) []byte {
	if reader.failed || length < 0 || length > len(reader.raw) {
		reader.failed = true
		return nil
	}
	value := reader.raw[:length]
	reader.raw = reader.raw[length:]
	return value
}

func (reader *byteReader) byte() byte {
	value := reader.take(1)
	if len(value) == 1 {
		reader.last = value[0]
		return value[0]
	}
	return 0
}

func (reader *byteReader) uint16() uint16 {
	value := reader.take(2)
	if len(value) != 2 {
		return 0
	}
	return uint16(value[0])<<8 | uint16(value[1])
}

func (reader *byteReader) uint64() uint64 {
	value := reader.take(8)
	if len(value) != 8 {
		return 0
	}
	var result uint64
	for _, byteValue := range value {
		result = result<<8 | uint64(byteValue)
	}
	return result
}

func (reader *byteReader) int64() int64 { return int64(reader.uint64()) }

func (reader *byteReader) short(maximum int) []byte {
	length := int(reader.byte())
	if length == 0 || length > maximum {
		reader.failed = true
		return nil
	}
	return reader.take(length)
}

func (reader *byteReader) done() bool { return !reader.failed && len(reader.raw) == 0 }

func signatureInput(body []byte) []byte {
	result := make([]byte, 0, len(inviteSignature)+1+len(body))
	result = append(result, inviteSignature...)
	result = append(result, 0)
	return append(result, body...)
}
