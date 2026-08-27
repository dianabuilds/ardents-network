package private

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
)

const profileDomain = "ardents-alpha-private-gateway-v1"

func signProfile(profile GatewayProfile, private ed25519.PrivateKey) []byte {
	return ed25519.Sign(private, profileTranscript(profile))
}

func validProfile(profile GatewayProfile, public ed25519.PublicKey) bool {
	return len(public) == ed25519.PublicKeySize && len(profile.Signature) == ed25519.SignatureSize &&
		profile.KeyConfigDigest == sha256.Sum256(profile.KeyConfig) && len(profile.KeyConfig) > 0 &&
		ed25519.Verify(public, profileTranscript(profile), profile.Signature)
}

func profileTranscript(profile GatewayProfile) []byte {
	out := make([]byte, 0, 2+len(profileDomain)+32+1+len(profile.Cohort)+32+1+len(profile.Family)+8+4+len(profile.KeyConfig))
	out = binary.BigEndian.AppendUint16(out, uint16(len(profileDomain)))
	out = append(out, profileDomain...)
	out = append(out, profile.NetworkID[:]...)
	out = append(out, byte(len(profile.Cohort)))
	out = append(out, profile.Cohort...)
	out = append(out, profile.NodeID[:]...)
	out = append(out, byte(len(profile.Family)))
	out = append(out, profile.Family...)
	out = binary.BigEndian.AppendUint64(out, uint64(profile.AssignmentNotAfter.UnixNano()))
	out = binary.BigEndian.AppendUint32(out, uint32(len(profile.KeyConfig)))
	return append(out, profile.KeyConfig...)
}
