package bridge

import (
	"bytes"
	"crypto/sha256"
)

const (
	inviteMagic       = "ardents-h3-bi1"
	maximumInviteFile = 4096
	signatureTag      = "ardents-h3-bridge-invite-signature-v1"
	inviteIDTag       = "ardents-h3-bridge-invite-id-v1"
)

type invite struct {
	body               []byte
	signature          []byte
	id                 [32]byte
	networkID          [32]byte
	epoch              uint64
	epochDigest        [32]byte
	profile            string
	roleDomain         byte
	identity           [32]byte
	family             [32]byte
	recordDigest       [32]byte
	domainProof        []byte
	assignmentNotAfter int64
	notBefore          int64
	notAfter           int64
	slotGeneration     byte
	slot               byte
	replaces           *[32]byte
	candidate          []byte
	issuerID           [32]byte
	commitment         [32]byte
	adapterProfile     string
}

func decodeInvite(raw []byte) (invite, Class) {
	if len(raw) == 0 || len(raw) > maximumInviteFile {
		return invite{}, classInvalid
	}
	outer := binaryDecoder{raw: raw}
	if !bytes.Equal(outer.take(len(inviteMagic)), []byte(inviteMagic)) {
		return invite{}, classInvalid
	}
	body := outer.medium(1, maximumInviteFile)
	signature := outer.take(64)
	if outer.failed || !outer.done() {
		return invite{}, classInvalid
	}
	reader := binaryDecoder{raw: body}
	if reader.uint16() != 1 {
		if reader.failed {
			return invite{}, classInvalid
		}
		return invite{}, classIncompatible
	}
	decoded := invite{body: bytes.Clone(body), signature: bytes.Clone(signature)}
	copy(decoded.networkID[:], reader.take(32))
	decoded.epoch = reader.uint64()
	copy(decoded.epochDigest[:], reader.take(32))
	profile := reader.short(1, 63)
	decoded.profile = string(profile)
	decoded.roleDomain = reader.byte()
	copy(decoded.identity[:], reader.take(32))
	copy(decoded.family[:], reader.take(32))
	copy(decoded.recordDigest[:], reader.take(32))
	decoded.domainProof = bytes.Clone(reader.medium(1, 512))
	decoded.assignmentNotAfter = reader.int64()
	decoded.notBefore = reader.int64()
	decoded.notAfter = reader.int64()
	decoded.slotGeneration = reader.byte()
	decoded.slot = reader.byte()
	present := reader.byte()
	if present == 1 {
		decoded.replaces = new([32]byte)
		copy(decoded.replaces[:], reader.take(32))
	} else if present != 0 {
		reader.failed = true
	}
	decoded.candidate = bytes.Clone(reader.medium(1, 1024))
	copy(decoded.issuerID[:], reader.take(32))
	if reader.failed || !reader.done() || !ascii(decoded.profile) {
		return invite{}, classInvalid
	}
	hash := sha256.New()
	hash.Write([]byte(inviteIDTag))
	hash.Write([]byte{0})
	hash.Write(body)
	copy(decoded.id[:], hash.Sum(nil))
	return decoded, classAccepted
}

func ascii(value string) bool {
	for _, character := range []byte(value) {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return value != ""
}
