package nameresolution

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
)

const gatewayProfileDomain = "ardents-naming-gateway-v1"

func signGatewayProfile(profile GatewayProfile, private ed25519.PrivateKey) []byte {
	return ed25519.Sign(private, gatewayProfileTranscript(profile))
}

func validGatewayProfile(profile GatewayProfile, public [32]byte) bool {
	return len(profile.Signature) == ed25519.SignatureSize &&
		profile.KeyConfigDigest == sha256.Sum256(profile.KeyConfig) && len(profile.KeyConfig) > 0 &&
		ed25519.Verify(ed25519.PublicKey(public[:]), gatewayProfileTranscript(profile), profile.Signature)
}

func gatewayProfileTranscript(profile GatewayProfile) []byte {
	out := make([]byte, 0, 2+len(gatewayProfileDomain)+32+32+8+4+len(profile.KeyConfig))
	out = binary.BigEndian.AppendUint16(out, uint16(len(gatewayProfileDomain)))
	out = append(out, gatewayProfileDomain...)
	out = append(out, profile.NetworkID[:]...)
	out = append(out, profile.NodeID[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(profile.AssignmentNotAfter.UnixNano()))
	out = binary.BigEndian.AppendUint32(out, uint32(len(profile.KeyConfig)))
	return append(out, profile.KeyConfig...)
}
