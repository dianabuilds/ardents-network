package publication

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"time"
)

const (
	publishCapability  = uint32(1)
	connectCapability  = uint32(2)
	credentialBodySize = 2 + 32 + 32 + 32 + 8 + 8 + 8 + 32 + 4
	credentialSize     = credentialBodySize + ed25519.SignatureSize
)

// Issue signs a bounded public delegation for one exclusive Service Instance.
func (value Credential) Issue(authority ed25519.PrivateKey) (Credential, error) {
	if len(authority) != ed25519.PrivateKeySize || value.InstancePublic == [32]byte{} || value.Generation == 0 ||
		value.NotBefore >= value.NotAfter || value.NetworkID == [32]byte{} || value.Capabilities == 0 {
		return Credential{}, errors.New("publication credential request is invalid")
	}
	public, ok := authority.Public().(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize {
		return Credential{}, errors.New("publication Authority key is invalid")
	}
	var authorityPublic [32]byte
	copy(authorityPublic[:], public)
	if value.AuthorityPublic != [32]byte{} && subtle.ConstantTimeCompare(value.AuthorityPublic[:], authorityPublic[:]) != 1 {
		return Credential{}, errors.New("publication credential Authority does not match signer")
	}
	value.AuthorityPublic, value.Target, value.Signature = authorityPublic, targetFor(authorityPublic), [64]byte{}
	copy(value.Signature[:], ed25519.Sign(authority, credentialBody(value)))
	return value, nil
}

func validateCredential(value Credential, authority, network [32]byte, at time.Time, capability uint32) error {
	if value.AuthorityPublic != authority || value.Target != targetFor(authority) || value.NetworkID != network ||
		value.InstancePublic == [32]byte{} || value.Generation == 0 || value.NotBefore >= value.NotAfter ||
		at.Unix() < value.NotBefore || at.Unix() >= value.NotAfter || value.Capabilities&capability != capability ||
		!ed25519.Verify(ed25519.PublicKey(authority[:]), credentialBody(value), value.Signature[:]) {
		return errors.New("publication Credential is not current for the exact Target")
	}
	return nil
}

// Validate verifies that a Credential grants the required capability for the
// exact Authority, Network, and decision time.
func Validate(value Credential, authority, network [32]byte, at time.Time, capability uint32) error {
	return validateCredential(value, authority, network, at, capability)
}

// Target returns the exact opaque Service Target for an Authority public key.
func Target(authority [32]byte) [32]byte { return targetFor(authority) }

func targetFor(authority [32]byte) [32]byte {
	return sha256.Sum256(append([]byte("ardents-service-target-v1\x00"), authority[:]...))
}

func credentialBody(value Credential) []byte {
	encoded := make([]byte, credentialBodySize)
	binary.BigEndian.PutUint16(encoded[:2], 1)
	offset := 2
	for _, field := range [][32]byte{value.AuthorityPublic, value.Target, value.InstancePublic} {
		copy(encoded[offset:offset+32], field[:])
		offset += 32
	}
	binary.BigEndian.PutUint64(encoded[offset:offset+8], value.Generation)
	offset += 8
	binary.BigEndian.PutUint64(encoded[offset:offset+8], uint64(value.NotBefore))
	offset += 8
	binary.BigEndian.PutUint64(encoded[offset:offset+8], uint64(value.NotAfter))
	offset += 8
	copy(encoded[offset:offset+32], value.NetworkID[:])
	offset += 32
	binary.BigEndian.PutUint32(encoded[offset:offset+4], value.Capabilities)
	return encoded
}

func encodeCredential(value Credential) []byte {
	encoded := make([]byte, credentialSize)
	copy(encoded, credentialBody(value))
	copy(encoded[credentialBodySize:], value.Signature[:])
	return encoded
}

func decodeCredential(encoded []byte) (Credential, error) {
	if len(encoded) != credentialSize || binary.BigEndian.Uint16(encoded[:2]) != 1 {
		return Credential{}, errors.New("publication Credential encoding is malformed")
	}
	var value Credential
	offset := 2
	for _, field := range []*[32]byte{&value.AuthorityPublic, &value.Target, &value.InstancePublic} {
		copy(field[:], encoded[offset:offset+32])
		offset += 32
	}
	value.Generation = binary.BigEndian.Uint64(encoded[offset : offset+8])
	offset += 8
	value.NotBefore = int64(binary.BigEndian.Uint64(encoded[offset : offset+8]))
	offset += 8
	value.NotAfter = int64(binary.BigEndian.Uint64(encoded[offset : offset+8]))
	offset += 8
	copy(value.NetworkID[:], encoded[offset:offset+32])
	offset += 32
	value.Capabilities = binary.BigEndian.Uint32(encoded[offset : offset+4])
	copy(value.Signature[:], encoded[credentialBodySize:])
	return value, nil
}
