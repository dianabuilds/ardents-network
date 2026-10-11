package publication

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
)

// signFixtureCredential is an independent test byte oracle, not a product
// issuer. Successful new Custody acceptance is exercised by the process profile.
func signFixtureCredential(value Credential, authority ed25519.PrivateKey) (Credential, error) {
	copy(value.AuthorityPublic[:], authority.Public().(ed25519.PublicKey))
	value.Target = sha256.Sum256(append([]byte("ardents-service-target-v3\x00"), value.AuthorityPublic[:]...))
	raw := fixtureCredentialBody(value)
	copy(value.Signature[:], ed25519.Sign(authority, raw))
	return value, nil
}

func fixtureCredentialBody(value Credential) []byte {
	raw := binary.BigEndian.AppendUint16(nil, 3)
	for _, field := range [][32]byte{value.AuthorityPublic, value.Target, value.InstancePublic} {
		raw = append(raw, field[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, value.Generation)
	raw = binary.BigEndian.AppendUint64(raw, uint64(value.NotBefore))
	raw = binary.BigEndian.AppendUint64(raw, uint64(value.NotAfter))
	raw = append(raw, value.NetworkID[:]...)
	return binary.BigEndian.AppendUint32(raw, value.Capabilities)
}
