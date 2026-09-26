package reachability

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

const (
	descriptorV1 = uint16(1)
	descriptorV2 = uint16(2)

	descriptorPrefixV1 = "ardents-reachability-descriptor-v1\x00"
	descriptorPrefixV2 = "ardents-reachability-descriptor-v2\x00"
)

// Verify decodes one closed supported Descriptor and proves that it names the
// expected Target under the declared Network at the supplied decision time.
func Verify(raw []byte, expectedTarget, network [32]byte, at time.Time) (Verified, error) {
	if expectedTarget == [32]byte{} || network == [32]byte{} || at.IsZero() || len(raw) == 0 || len(raw) > MaximumDescriptorSize {
		return Verified{}, errors.New("reachability descriptor verification input is invalid")
	}
	descriptor, body, err := decode(raw)
	if err != nil || descriptor.NetworkID != network || descriptor.Target != expectedTarget ||
		publication.Target(descriptor.AuthorityPublic) != expectedTarget {
		return Verified{}, errors.New("reachability descriptor target binding is invalid")
	}
	current, err := publication.Decode(descriptor.Publication, ed25519.PublicKey(descriptor.AuthorityPublic[:]), network, at)
	if err != nil || current.Credential.Target != expectedTarget || current.Credential.AuthorityPublic != descriptor.AuthorityPublic ||
		current.Digest != descriptor.PublicationDigest || !validIntroduction(descriptor.Introduction, current.Credential.NotAfter) ||
		!at.Before(descriptor.Introduction.NotAfter) {
		return Verified{}, errors.New("reachability descriptor publication or Introduction is invalid")
	}
	commitment := sha256.Sum256(append([]byte(descriptorPrefix(descriptor.Version)), body...))
	if !ed25519.Verify(ed25519.PublicKey(current.Credential.InstancePublic[:]), commitment[:], descriptor.Signature[:]) {
		return Verified{}, errors.New("reachability descriptor Instance signature is invalid")
	}
	return Verified{Descriptor: cloneDescriptor(descriptor), Current: current}, nil
}

func validIntroduction(value Introduction, credentialNotAfter int64) bool {
	return value.StateDigest != [32]byte{} && value.Epoch != 0 && value.IntroductionNodeID != [32]byte{} &&
		value.RendezvousNodeID != [32]byte{} && value.IntroductionNodeID != value.RendezvousNodeID &&
		value.Reachability != [32]byte{} && value.JoinHandle != [32]byte{} && !value.NotAfter.IsZero() &&
		value.NotAfter.Equal(value.NotAfter.UTC().Truncate(time.Second)) && value.NotAfter.Unix() <= credentialNotAfter &&
		validSubmission(value)
}

func validSubmission(value Introduction) bool {
	switch value.SubmissionMode {
	case SubmissionFixedGrant:
		return len(value.SubmissionAuthorization) > 0 && len(value.SubmissionAuthorization) <= maximumAuthorization
	case SubmissionMembershipGrant:
		return len(value.SubmissionAuthorization) == 0
	default:
		return false
	}
}

func decode(raw []byte) (Descriptor, []byte, error) {
	if len(raw) < 2+32*9+8+8+2+2+ed25519.SignatureSize {
		return Descriptor{}, nil, errors.New("reachability descriptor encoding is malformed")
	}
	body, signature := raw[:len(raw)-ed25519.SignatureSize], raw[len(raw)-ed25519.SignatureSize:]
	offset := 0
	readU16 := func() (uint16, bool) {
		if offset+2 > len(body) {
			return 0, false
		}
		value := binary.BigEndian.Uint16(body[offset : offset+2])
		offset += 2
		return value, true
	}
	version, ok := readU16()
	if !ok || (version != descriptorV1 && version != descriptorV2) {
		return Descriptor{}, nil, errors.New("reachability descriptor version is unsupported")
	}
	var fields [9][32]byte
	for index := range fields {
		if offset+32 > len(body) {
			return Descriptor{}, nil, errors.New("reachability descriptor field is truncated")
		}
		copy(fields[index][:], body[offset:offset+32])
		offset += 32
	}
	if offset+8+8 > len(body) {
		return Descriptor{}, nil, errors.New("reachability descriptor timing is truncated")
	}
	epoch := binary.BigEndian.Uint64(body[offset : offset+8])
	offset += 8
	notAfter := int64(binary.BigEndian.Uint64(body[offset : offset+8]))
	offset += 8
	submissionMode := SubmissionFixedGrant
	if version == descriptorV2 {
		if offset >= len(body) {
			return Descriptor{}, nil, errors.New("reachability descriptor submission mode is truncated")
		}
		submissionMode = SubmissionMode(body[offset])
		offset++
	}
	authorizationLength, ok := readU16()
	if !ok {
		return Descriptor{}, nil, errors.New("reachability descriptor authorization is truncated")
	}
	publicationLength, ok := readU16()
	if !ok || int(authorizationLength) > maximumAuthorization || int(publicationLength) > MaximumDescriptorSize {
		return Descriptor{}, nil, errors.New("reachability descriptor length is invalid")
	}
	if offset+int(authorizationLength)+int(publicationLength) != len(body) {
		return Descriptor{}, nil, errors.New("reachability descriptor trailing bytes are invalid")
	}
	descriptor := Descriptor{Version: version, NetworkID: fields[0], Target: fields[1], AuthorityPublic: fields[2], PublicationDigest: fields[3],
		Introduction: Introduction{StateDigest: fields[4], IntroductionNodeID: fields[5], RendezvousNodeID: fields[6], Reachability: fields[7], JoinHandle: fields[8], Epoch: epoch, NotAfter: time.Unix(notAfter, 0).UTC()}}
	descriptor.Introduction.SubmissionMode = submissionMode
	descriptor.Introduction.SubmissionAuthorization = append([]byte(nil), body[offset:offset+int(authorizationLength)]...)
	offset += int(authorizationLength)
	descriptor.Publication = append([]byte(nil), body[offset:offset+int(publicationLength)]...)
	copy(descriptor.Signature[:], signature)
	if descriptor.NetworkID == [32]byte{} || descriptor.Target == [32]byte{} || descriptor.AuthorityPublic == [32]byte{} || descriptor.PublicationDigest == [32]byte{} || !validIntroduction(descriptor.Introduction, descriptor.Introduction.NotAfter.Unix()) {
		return Descriptor{}, nil, errors.New("reachability descriptor content is invalid")
	}
	return descriptor, append([]byte(nil), body...), nil
}

func descriptorPrefix(version uint16) string {
	if version == descriptorV2 {
		return descriptorPrefixV2
	}
	return descriptorPrefixV1
}

func cloneIntroduction(value Introduction) Introduction {
	value.SubmissionAuthorization = append([]byte(nil), value.SubmissionAuthorization...)
	return value
}

func cloneDescriptor(value Descriptor) Descriptor {
	value.Publication = append([]byte(nil), value.Publication...)
	value.Introduction = cloneIntroduction(value.Introduction)
	return value
}
