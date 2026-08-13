package serviceconn

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"time"
)

const credentialBodySize = 4 + 1 + 32 + 32 + 32 + 8 + 8 + 8 + 32 + 4
const credentialSize = credentialBodySize + ed25519.SignatureSize

// IssueCredential signs one bounded laboratory Credential with Service Authority.
func IssueCredential(authority ed25519.PrivateKey, value Credential) (Credential, error) {
	if len(authority) != ed25519.PrivateKeySize || value.InstancePublic == [32]byte{} ||
		value.Generation == 0 || value.NotBefore >= value.NotAfter || value.NetworkID == [32]byte{} || value.Capabilities == 0 {
		return Credential{}, errors.New("credential request is invalid")
	}
	public, ok := authority.Public().(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize {
		return Credential{}, errors.New("service Authority key is invalid")
	}
	var authorityPublic [32]byte
	copy(authorityPublic[:], public)
	if value.AuthorityPublic != [32]byte{} && subtle.ConstantTimeCompare(value.AuthorityPublic[:], authorityPublic[:]) != 1 {
		return Credential{}, errors.New("credential Authority does not match signer")
	}
	value.AuthorityPublic = authorityPublic
	value.Target = deriveTarget(authorityPublic)
	value.Signature = [64]byte{}
	signature := ed25519.Sign(authority, credentialBody(value))
	copy(value.Signature[:], signature)
	return value, nil
}

func validateCredential(value Credential, authority, network [32]byte, at time.Time, capability uint32) error {
	if value.AuthorityPublic != authority || value.Target != deriveTarget(authority) || value.NetworkID != network ||
		value.InstancePublic == [32]byte{} || value.Generation == 0 || value.NotBefore >= value.NotAfter ||
		at.Unix() < value.NotBefore || at.Unix() >= value.NotAfter || value.Capabilities&capability != capability ||
		!ed25519.Verify(ed25519.PublicKey(authority[:]), credentialBody(value), value.Signature[:]) {
		return errors.New("service Credential is not current for the exact Target")
	}
	return nil
}

func deriveTarget(authority [32]byte) [32]byte {
	value := make([]byte, 0, 30+len(authority))
	value = append(value, "ardents-h3-service-target-v1\x00"...)
	value = append(value, authority[:]...)
	return sha256.Sum256(value)
}

func credentialBody(value Credential) []byte {
	encoded := make([]byte, credentialBodySize)
	copy(encoded[:4], "ASCR")
	encoded[4] = 1
	offset := 5
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
	if len(encoded) != credentialSize || string(encoded[:4]) != "ASCR" || encoded[4] != 1 {
		return Credential{}, errors.New("credential encoding is malformed")
	}
	var value Credential
	offset := 5
	fields := []*[32]byte{&value.AuthorityPublic, &value.Target, &value.InstancePublic}
	for _, field := range fields {
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
