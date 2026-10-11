package publication

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"
)

// Credential is sealed verified public delegation. It grants no private key,
// generic signer, floor reconciliation, live registration or readiness.
type Credential struct {
	delegation Delegation
	raw        [credentialSize]byte
}

func (credential Credential) Delegation() Delegation { return credential.delegation }
func (credential Credential) Digest() [32]byte       { return sha256.Sum256(credential.raw[:]) }

// VerifyCredential requires both Publish and Connect for an exact independently
// selected Target/Network. Instance response acceptance adds its exact request
// and at-most-once checks; public signature verification alone cannot bind it.
func VerifyCredential(raw []byte, target, network [32]byte, at time.Time) (Credential, error) {
	value, err := verifyCredential(raw, target, network, at, publishCapability|connectCapability)
	if err != nil {
		return Credential{}, err
	}
	result := Credential{delegation: value}
	copy(result.raw[:], raw)
	return result, nil
}

func verifyCredential(raw []byte, target, network [32]byte, at time.Time, required uint32) (Delegation, error) {
	if len(raw) != credentialSize || target == [32]byte{} || network == [32]byte{} || at.IsZero() || binary.BigEndian.Uint16(raw[:2]) != 3 {
		return Delegation{}, errors.New("publication Credential version or input invalid")
	}
	var value Delegation
	copy(value.Authority[:], raw[2:34])
	copy(value.Target[:], raw[34:66])
	copy(value.Instance[:], raw[66:98])
	value.Generation = binary.BigEndian.Uint64(raw[98:106])
	before, after := binary.BigEndian.Uint64(raw[106:114]), binary.BigEndian.Uint64(raw[114:122])
	copy(value.Network[:], raw[122:154])
	caps := binary.BigEndian.Uint32(raw[154:158])
	if before > 1<<63-1 || after > 1<<63-1 || before >= after || value.Authority == [32]byte{} || value.Instance == [32]byte{} || value.Generation == 0 || value.Network != network || value.Target != target || Target(value.Authority) != target || caps&required != required {
		return Delegation{}, errors.New("publication Credential binding or capability invalid")
	}
	value.NotBefore, value.NotAfter = time.Unix(int64(before), 0).UTC(), time.Unix(int64(after), 0).UTC()
	if at.Before(value.NotBefore) || !at.Before(value.NotAfter) || !ed25519.Verify(ed25519.PublicKey(value.Authority[:]), raw[:credentialBodySize], raw[credentialBodySize:]) {
		return Delegation{}, errors.New("publication Credential validity or signature invalid")
	}
	return value, nil
}
