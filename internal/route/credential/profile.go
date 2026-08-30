package credential

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"
)

const (
	legacyProfileVersion = byte(1)
	grantProfileVersion  = byte(2)
	profileVersion       = byte(3)
	legacyProfileDomain  = "ardents-transit-issuance-profile-v1"
	grantProfileDomain   = "ardents-transit-issuance-profile-v2"
	profileDomain        = "ardents-transit-issuance-profile-v3"
)

// EncodeProfile serializes one signed issuer OHTTP profile. State treats the
// bytes as opaque; callers must verify it against State's selected Node key.
func EncodeProfile(profile Profile) ([]byte, error) {
	if !validProfile(profile) {
		return nil, errors.New("transit issuance profile is invalid")
	}
	out := make([]byte, 0, 4+1+32*4+8+2+len(profile.KeyConfig)+32+ed25519.SignatureSize)
	out = append(out, "ATIP"...)
	out = append(out, profile.Version)
	for _, value := range [][32]byte{profile.NetworkID, profile.NodeID, profile.GrantSignerID} {
		out = append(out, value[:]...)
	}
	if profile.Version >= grantProfileVersion {
		out = append(out, profile.GrantSignerPublicKey[:]...)
	}
	if profile.Version == profileVersion {
		out = append(out, profile.InitiatorNodeID[:]...)
		out = append(out, profile.InitiatorPublicKey[:]...)
	}
	out = binary.BigEndian.AppendUint64(out, uint64(profile.AssignmentNotAfter.UnixNano()))
	out = binary.BigEndian.AppendUint16(out, uint16(len(profile.KeyConfig)))
	out = append(out, profile.KeyConfig...)
	out = append(out, profile.KeyConfigDigest[:]...)
	return append(out, profile.Signature...), nil
}

// DecodeProfile decodes bounded opaque State profile bytes without selecting
// or trusting their Node association.
func DecodeProfile(raw []byte) (Profile, error) {
	const legacyFixed = 4 + 1 + 32 + 32 + 32 + 8 + 2 + 32 + ed25519.SignatureSize
	if len(raw) < legacyFixed || len(raw) > maximumProfileBytes || string(raw[:4]) != "ATIP" ||
		(raw[4] != legacyProfileVersion && raw[4] != grantProfileVersion && raw[4] != profileVersion) {
		return Profile{}, errors.New("transit issuance profile encoding is invalid")
	}
	offset := 5
	profile := Profile{Version: raw[4]}
	for _, destination := range []*[32]byte{&profile.NetworkID, &profile.NodeID, &profile.GrantSignerID} {
		copy(destination[:], raw[offset:offset+32])
		offset += 32
	}
	if profile.Version >= grantProfileVersion {
		if len(raw) < legacyFixed+32 {
			return Profile{}, errors.New("transit issuance profile encoding is invalid")
		}
		copy(profile.GrantSignerPublicKey[:], raw[offset:offset+32])
		offset += 32
	}
	if profile.Version == profileVersion {
		if len(raw) < legacyFixed+32*3 {
			return Profile{}, errors.New("transit issuance profile encoding is invalid")
		}
		copy(profile.InitiatorNodeID[:], raw[offset:offset+32])
		offset += 32
		copy(profile.InitiatorPublicKey[:], raw[offset:offset+32])
		offset += 32
	}
	profile.AssignmentNotAfter = time.Unix(0, int64(binary.BigEndian.Uint64(raw[offset:offset+8]))).UTC()
	offset += 8
	keyLength := int(binary.BigEndian.Uint16(raw[offset : offset+2]))
	offset += 2
	if keyLength == 0 || offset+keyLength+32+ed25519.SignatureSize != len(raw) {
		return Profile{}, errors.New("transit issuance profile encoding is malformed")
	}
	profile.KeyConfig = append([]byte(nil), raw[offset:offset+keyLength]...)
	offset += keyLength
	copy(profile.KeyConfigDigest[:], raw[offset:offset+32])
	offset += 32
	profile.Signature = append([]byte(nil), raw[offset:]...)
	if !validProfile(profile) {
		return Profile{}, errors.New("transit issuance profile content is invalid")
	}
	return profile, nil
}

// VerifyProfile verifies one profile's OHTTP material against exactly the
// State-selected issuer Node and bounded operation window.
func VerifyProfile(profile Profile, network, node, public [32]byte, at, deadline time.Time) error {
	if network == [32]byte{} || node == [32]byte{} || public == [32]byte{} || at.IsZero() || !at.Before(deadline) ||
		deadline.After(at.Add(15*time.Second)) || profile.NetworkID != network || profile.NodeID != node ||
		profile.AssignmentNotAfter.Before(deadline) || !validProfile(profile) ||
		!ed25519.Verify(ed25519.PublicKey(public[:]), profileTranscript(profile), profile.Signature) {
		return errors.New("transit issuance profile does not match State")
	}
	return nil
}

func validProfile(profile Profile) bool {
	versionValid := profile.Version == legacyProfileVersion && profile.GrantSignerPublicKey == [32]byte{} &&
		profile.InitiatorNodeID == [32]byte{} && profile.InitiatorPublicKey == [32]byte{} ||
		profile.Version == grantProfileVersion && profile.GrantSignerPublicKey != [32]byte{} &&
			profile.GrantSignerID == sha256.Sum256(profile.GrantSignerPublicKey[:]) &&
			profile.InitiatorNodeID == [32]byte{} && profile.InitiatorPublicKey == [32]byte{} ||
		profile.Version == profileVersion && profile.GrantSignerPublicKey != [32]byte{} &&
			profile.GrantSignerID == sha256.Sum256(profile.GrantSignerPublicKey[:]) &&
			profile.InitiatorNodeID != [32]byte{} && profile.InitiatorPublicKey != [32]byte{}
	return versionValid && profile.NetworkID != [32]byte{} && profile.NodeID != [32]byte{} && profile.GrantSignerID != [32]byte{} &&
		!profile.AssignmentNotAfter.IsZero() && len(profile.KeyConfig) > 0 && len(profile.KeyConfig) <= maximumProfileBytes &&
		profile.KeyConfigDigest == sha256.Sum256(profile.KeyConfig) && len(profile.Signature) == ed25519.SignatureSize
}

func profileTranscript(profile Profile) []byte {
	domain := profileDomain
	if profile.Version == legacyProfileVersion {
		domain = legacyProfileDomain
	} else if profile.Version == grantProfileVersion {
		domain = grantProfileDomain
	}
	out := make([]byte, 0, 2+len(domain)+32*4+8+4+len(profile.KeyConfig))
	out = binary.BigEndian.AppendUint16(out, uint16(len(domain)))
	out = append(out, domain...)
	for _, value := range [][32]byte{profile.NetworkID, profile.NodeID, profile.GrantSignerID} {
		out = append(out, value[:]...)
	}
	if profile.Version >= grantProfileVersion {
		out = append(out, profile.GrantSignerPublicKey[:]...)
	}
	if profile.Version == profileVersion {
		out = append(out, profile.InitiatorNodeID[:]...)
		out = append(out, profile.InitiatorPublicKey[:]...)
	}
	out = binary.BigEndian.AppendUint64(out, uint64(profile.AssignmentNotAfter.UnixNano()))
	out = binary.BigEndian.AppendUint32(out, uint32(len(profile.KeyConfig)))
	return append(out, profile.KeyConfig...)
}
