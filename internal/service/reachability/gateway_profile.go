package reachability

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"
)

const (
	gatewayProfileVersion      = byte(1)
	MaximumGatewayProfileBytes = 4096
)

// EncodeGatewayProfile returns the closed, self-contained form of one signed
// Gateway profile. Network State carries these opaque bytes; it does not own
// OHTTP key-profile interpretation.
func EncodeGatewayProfile(profile GatewayProfile) ([]byte, error) {
	if !validGatewayProfileFields(profile) {
		return nil, errors.New("private reachability Gateway profile is invalid")
	}
	out := make([]byte, 0, 4+1+32+32+8+2+len(profile.KeyConfig)+32+ed25519.SignatureSize)
	out = append(out, "ARGP"...)
	out = append(out, gatewayProfileVersion)
	out = append(out, profile.NetworkID[:]...)
	out = append(out, profile.NodeID[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(profile.AssignmentNotAfter.UnixNano()))
	out = binary.BigEndian.AppendUint16(out, uint16(len(profile.KeyConfig)))
	out = append(out, profile.KeyConfig...)
	out = append(out, profile.KeyConfigDigest[:]...)
	return append(out, profile.Signature...), nil
}

// DecodeGatewayProfile parses one bounded closed Gateway profile. A caller
// must still call VerifyGatewayProfile with State's selected identity and
// exact operation window before opening a private lookup.
func DecodeGatewayProfile(raw []byte) (GatewayProfile, error) {
	const fixed = 4 + 1 + 32 + 32 + 8 + 2 + 32 + ed25519.SignatureSize
	if len(raw) < fixed || len(raw) > MaximumGatewayProfileBytes || string(raw[:4]) != "ARGP" || raw[4] != gatewayProfileVersion {
		return GatewayProfile{}, errors.New("private reachability Gateway profile encoding is invalid")
	}
	offset := 5
	var profile GatewayProfile
	copy(profile.NetworkID[:], raw[offset:offset+32])
	offset += 32
	copy(profile.NodeID[:], raw[offset:offset+32])
	offset += 32
	profile.AssignmentNotAfter = time.Unix(0, int64(binary.BigEndian.Uint64(raw[offset:offset+8]))).UTC()
	offset += 8
	keyLength := int(binary.BigEndian.Uint16(raw[offset : offset+2]))
	offset += 2
	if keyLength == 0 || offset+keyLength+32+ed25519.SignatureSize != len(raw) {
		return GatewayProfile{}, errors.New("private reachability Gateway profile encoding is malformed")
	}
	profile.KeyConfig = append([]byte(nil), raw[offset:offset+keyLength]...)
	offset += keyLength
	copy(profile.KeyConfigDigest[:], raw[offset:offset+32])
	offset += 32
	profile.Signature = append([]byte(nil), raw[offset:]...)
	if !validGatewayProfileFields(profile) {
		return GatewayProfile{}, errors.New("private reachability Gateway profile content is invalid")
	}
	return profile, nil
}

// VerifyGatewayProfile verifies the signed OHTTP configuration against the
// exact Gateway identity and finite State-selected lookup window. It neither
// selects a Gateway nor supplies an endpoint literal.
func VerifyGatewayProfile(profile GatewayProfile, network, node, public [32]byte, at, deadline time.Time) error {
	if network == [32]byte{} || node == [32]byte{} || public == [32]byte{} || at.IsZero() || !at.Before(deadline) ||
		deadline.After(at.Add(15*time.Second)) || profile.NetworkID != network || profile.NodeID != node ||
		profile.AssignmentNotAfter.Before(deadline) || !validGatewayProfile(profile, public) {
		return errors.New("private reachability Gateway profile does not match State")
	}
	return nil
}

func validGatewayProfileFields(profile GatewayProfile) bool {
	return profile.NetworkID != [32]byte{} && profile.NodeID != [32]byte{} && len(profile.Signature) == ed25519.SignatureSize &&
		len(profile.KeyConfig) > 0 && len(profile.KeyConfig) <= MaximumGatewayProfileBytes &&
		profile.KeyConfigDigest == sha256.Sum256(profile.KeyConfig) && !profile.AssignmentNotAfter.IsZero()
}

func validGatewayProfile(profile GatewayProfile, public [32]byte) bool {
	return public != [32]byte{} && validGatewayProfileFields(profile) &&
		ed25519.Verify(ed25519.PublicKey(public[:]), gatewayProfileTranscript(profile), profile.Signature)
}

func gatewayProfileTranscript(profile GatewayProfile) []byte {
	out := make([]byte, 0, 2+len(privateGatewayProfileDomain)+32+32+8+4+len(profile.KeyConfig))
	out = binary.BigEndian.AppendUint16(out, uint16(len(privateGatewayProfileDomain)))
	out = append(out, privateGatewayProfileDomain...)
	out = append(out, profile.NetworkID[:]...)
	out = append(out, profile.NodeID[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(profile.AssignmentNotAfter.UnixNano()))
	out = binary.BigEndian.AppendUint32(out, uint32(len(profile.KeyConfig)))
	return append(out, profile.KeyConfig...)
}
