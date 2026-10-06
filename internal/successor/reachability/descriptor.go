package reachability

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

const MaximumDescriptorSize = 15000
const descriptorHeaderSize = 284

// Introduction is the Instance-signed recipient fact, without a Rendezvous,
// reusable join secret, origin address or accepting registration receipt.
type Introduction struct {
	Revision                 uint64
	Node, Slot, RecipientKey [32]byte
	NotBefore, NotAfter      time.Time
}

// Descriptor contains public signed facts. Store/history independently verify
// input bytes; possession or mutation of this value grants no authority.
type Descriptor struct {
	Network, Target, Authority, PublicationDigest, ProfileDigest [32]byte
	Introduction                                                 Introduction
	Publication                                                  publication.Proof
	raw                                                          []byte
}

func (descriptor Descriptor) Bytes() []byte    { return append([]byte(nil), descriptor.raw...) }
func (descriptor Descriptor) Digest() [32]byte { return sha256.Sum256(descriptor.raw) }

// Current checks the verified recipient interval at the caller's actual effect
// or handoff time. It reads retained signed bytes, not mutable exported facts;
// it grants no Network authority, registration receipt or Service readiness.
func (descriptor Descriptor) Current(at time.Time) error {
	if at.IsZero() || len(descriptor.raw) <= descriptorHeaderSize+ed25519.SignatureSize {
		return errors.New("reachability verified Descriptor absent")
	}
	before := time.Unix(int64(binary.BigEndian.Uint64(descriptor.raw[266:274])), 0).UTC()
	after := time.Unix(int64(binary.BigEndian.Uint64(descriptor.raw[274:282])), 0).UTC()
	if at.Before(before) || !at.Before(after) {
		return errors.New("reachability recipient not current")
	}
	return nil
}

// Verify requires the lookup's independently selected Target and current profile.
func Verify(raw []byte, target, network, profile [32]byte, at time.Time) (Descriptor, error) {
	return verifyDescriptor(raw, target, network, profile, at, false)
}

// VerifyPublish derives the public Target and requires Publish as well as
// Connect delegation. The receiver must still check current Introduction duty
// and commit its genuine Store before acknowledgement.
func VerifyPublish(raw []byte, network, profile [32]byte, at time.Time) (Descriptor, error) {
	if len(raw) < 66 {
		return Descriptor{}, errors.New("reachability Descriptor length invalid")
	}
	var target [32]byte
	copy(target[:], raw[34:66])
	return verifyDescriptor(raw, target, network, profile, at, true)
}

func verifyDescriptor(raw []byte, target, network, profile [32]byte, at time.Time, publish bool) (Descriptor, error) {
	if target == [32]byte{} || network == [32]byte{} || profile == [32]byte{} || at.IsZero() || len(raw) <= descriptorHeaderSize+ed25519.SignatureSize || len(raw) > MaximumDescriptorSize || binary.BigEndian.Uint16(raw[:2]) != 3 {
		return Descriptor{}, errors.New("reachability Descriptor input invalid")
	}
	var value Descriptor
	offset := 2
	for _, field := range []*[32]byte{&value.Network, &value.Target, &value.Authority, &value.PublicationDigest, &value.ProfileDigest} {
		copy(field[:], raw[offset:offset+32])
		offset += 32
	}
	if value.Network != network || value.Target != target || value.ProfileDigest != profile || publication.Target(value.Authority) != target {
		return Descriptor{}, errors.New("reachability Descriptor binding differs")
	}
	recipientStart := offset
	value.Introduction.Revision = binary.BigEndian.Uint64(raw[offset : offset+8])
	offset += 8
	for _, field := range []*[32]byte{&value.Introduction.Node, &value.Introduction.Slot, &value.Introduction.RecipientKey} {
		copy(field[:], raw[offset:offset+32])
		offset += 32
	}
	before, after := binary.BigEndian.Uint64(raw[offset:offset+8]), binary.BigEndian.Uint64(raw[offset+8:offset+16])
	offset += 16
	if before == 0 || before > 1<<63-1 || after > 1<<63-1 || before >= after || after-before > 600 || value.Introduction.Revision == 0 || value.Introduction.Node == [32]byte{} || value.Introduction.Slot == [32]byte{} || value.Introduction.RecipientKey == [32]byte{} {
		return Descriptor{}, errors.New("reachability recipient bounds invalid")
	}
	value.Introduction.NotBefore, value.Introduction.NotAfter = time.Unix(int64(before), 0).UTC(), time.Unix(int64(after), 0).UTC()
	if at.Before(value.Introduction.NotBefore) || !at.Before(value.Introduction.NotAfter) {
		return Descriptor{}, errors.New("reachability recipient not current")
	}
	recipientEnd := offset
	length := int(binary.BigEndian.Uint16(raw[offset : offset+2]))
	offset += 2
	if length == 0 || offset+length+ed25519.SignatureSize != len(raw) {
		return Descriptor{}, errors.New("reachability Publication length invalid")
	}
	record := raw[offset : offset+length]
	var err error
	if publish {
		value.Publication, err = publication.VerifyPublish(record, target, network, at)
	} else {
		value.Publication, err = publication.Verify(record, target, network, at)
	}
	if err != nil {
		return Descriptor{}, err
	}
	delegation := value.Publication.Delegation()
	if delegation.Authority != value.Authority || value.Publication.Digest() != value.PublicationDigest || value.Introduction.NotBefore.Before(delegation.NotBefore) || value.Introduction.NotAfter.After(delegation.NotAfter) {
		return Descriptor{}, errors.New("reachability Publication or recipient binding invalid")
	}
	transcript := []byte("ardents-private-reachability-v3\x00")
	for _, field := range [][32]byte{network, profile, value.PublicationDigest} {
		transcript = append(transcript, field[:]...)
	}
	transcript = append(transcript, raw[recipientStart:recipientEnd]...)
	if !ed25519.Verify(ed25519.PublicKey(delegation.Instance[:]), transcript, raw[offset+length:]) {
		return Descriptor{}, errors.New("reachability Instance signature invalid")
	}
	value.raw = append([]byte(nil), raw...)
	return value, nil
}
