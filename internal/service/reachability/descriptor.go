package reachability

import (
	"crypto"
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

// Issue creates the Descriptor v1 fixed-Grant or v2 membership-Grant
// encoding for one current Publication and live slot. It verifies the current
// Publication before granting the Instance signer authority to issue it.
func Issue(input IssueInput) ([]byte, Descriptor, error) {
	current, err := verifiedCurrent(input.Current)
	introduction := cloneIntroduction(input.Introduction)
	if introduction.SubmissionMode == 0 {
		introduction.SubmissionMode = SubmissionFixedGrant
	}
	if err != nil || input.InstanceSigner == nil || !validIntroduction(introduction, current.Credential.NotAfter) {
		return nil, Descriptor{}, errors.New("reachability descriptor issue input is invalid")
	}
	public, ok := input.InstanceSigner.Public().(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize || string(public) != string(current.Credential.InstancePublic[:]) {
		return nil, Descriptor{}, errors.New("reachability descriptor Instance signer does not match Publication")
	}
	var authority [32]byte
	copy(authority[:], current.Credential.AuthorityPublic[:])
	descriptor := Descriptor{NetworkID: current.Credential.NetworkID, Target: current.Credential.Target,
		AuthorityPublic: authority, Publication: append([]byte(nil), current.Record...), PublicationDigest: current.Digest,
		Introduction: introduction}
	if descriptor.Introduction.SubmissionMode == SubmissionFixedGrant {
		descriptor.Version = descriptorV1
	} else {
		descriptor.Version = descriptorV2
	}
	body, err := encodeBody(descriptor)
	if err != nil {
		return nil, Descriptor{}, err
	}
	commitment := sha256.Sum256(append([]byte(descriptorPrefix(descriptor.Version)), body...))
	signature, err := input.InstanceSigner.Sign(nil, commitment[:], crypto.Hash(0))
	if err != nil || len(signature) != ed25519.SignatureSize {
		return nil, Descriptor{}, errors.New("reachability descriptor Instance signer failed")
	}
	copy(descriptor.Signature[:], signature)
	raw := append(body, descriptor.Signature[:]...)
	if len(raw) > MaximumDescriptorSize {
		return nil, Descriptor{}, errors.New("reachability descriptor exceeds bound")
	}
	return raw, cloneDescriptor(descriptor), nil
}

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

func verifiedCurrent(value publication.Current) (publication.Current, error) {
	if value.Credential.AuthorityPublic == [32]byte{} || value.Credential.NetworkID == [32]byte{} || len(value.Record) == 0 {
		return publication.Current{}, errors.New("publication is incomplete")
	}
	return publication.Decode(value.Record, ed25519.PublicKey(value.Credential.AuthorityPublic[:]), value.Credential.NetworkID,
		time.Unix(value.Credential.NotBefore, 0).UTC())
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

func encodeBody(value Descriptor) ([]byte, error) {
	if value.NetworkID == [32]byte{} || value.Target == [32]byte{} || value.AuthorityPublic == [32]byte{} ||
		value.PublicationDigest == [32]byte{} || len(value.Publication) == 0 || len(value.Publication) > MaximumDescriptorSize ||
		!validIntroduction(value.Introduction, value.Introduction.NotAfter.Unix()) ||
		(value.Version != descriptorV1 && value.Version != descriptorV2) ||
		(value.Version == descriptorV1 && value.Introduction.SubmissionMode != SubmissionFixedGrant) ||
		(value.Version == descriptorV2 && value.Introduction.SubmissionMode != SubmissionMembershipGrant) {
		return nil, errors.New("reachability descriptor body is invalid")
	}
	if len(value.Publication) > 0xffff || len(value.Introduction.SubmissionAuthorization) > 0xffff {
		return nil, errors.New("reachability descriptor field exceeds encoding")
	}
	body := make([]byte, 0, 2+32*9+8+8+2+2+len(value.Publication)+len(value.Introduction.SubmissionAuthorization))
	var version [2]byte
	binary.BigEndian.PutUint16(version[:], value.Version)
	body = append(body, version[:]...)
	for _, field := range [][32]byte{value.NetworkID, value.Target, value.AuthorityPublic, value.PublicationDigest,
		value.Introduction.StateDigest, value.Introduction.IntroductionNodeID, value.Introduction.RendezvousNodeID,
		value.Introduction.Reachability, value.Introduction.JoinHandle} {
		body = append(body, field[:]...)
	}
	var epoch [8]byte
	binary.BigEndian.PutUint64(epoch[:], value.Introduction.Epoch)
	body = append(body, epoch[:]...)
	var notAfter [8]byte
	binary.BigEndian.PutUint64(notAfter[:], uint64(value.Introduction.NotAfter.Unix()))
	body = append(body, notAfter[:]...)
	if value.Version == descriptorV2 {
		body = append(body, byte(value.Introduction.SubmissionMode))
	}
	var length [2]byte
	binary.BigEndian.PutUint16(length[:], uint16(len(value.Introduction.SubmissionAuthorization)))
	body = append(body, length[:]...)
	binary.BigEndian.PutUint16(length[:], uint16(len(value.Publication)))
	body = append(body, length[:]...)
	body = append(body, value.Introduction.SubmissionAuthorization...)
	body = append(body, value.Publication...)
	return body, nil
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
