package publication

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"
)

const (
	publicationDomain  = "ardents-service-publication-v3\x00"
	credentialBodySize = 158
	credentialSize     = credentialBodySize + ed25519.SignatureSize
	publicProofSize    = len(publicationDomain) + credentialSize + 32 + ed25519.SignatureSize
	publishCapability  = uint32(1)
	connectCapability  = uint32(2)
)

// Delegation is copied public evidence, never a live Instance or signing right.
type Delegation struct {
	Authority, Target, Instance, Network [32]byte
	Generation                           uint64
	NotBefore, NotAfter                  time.Time
}

// Proof retains copied verified public facts and their canonical digest.
// Verification does not attest the readiness committed by a separate Publisher.
type Proof struct {
	delegation Delegation
	digest     [32]byte
}

func (proof Proof) Delegation() Delegation { return proof.delegation }
func (proof Proof) Digest() [32]byte       { return proof.digest }

// Target derives the accepted v3 opaque Service Target from its Authority key.
func Target(authority [32]byte) [32]byte {
	return sha256.Sum256(append([]byte("ardents-service-target-v3\x00"), authority[:]...))
}

// Verify checks public read evidence for an independently selected Target.
func Verify(record []byte, target, network [32]byte, at time.Time) (Proof, error) {
	return verify(record, target, network, at, connectCapability)
}

// VerifyPublish also requires delegated publication authority. A Connect-only
// signed proof cannot authorize receiving Store mutation.
func VerifyPublish(record []byte, target, network [32]byte, at time.Time) (Proof, error) {
	return verify(record, target, network, at, publishCapability|connectCapability)
}

func verify(record []byte, target, network [32]byte, at time.Time, required uint32) (Proof, error) {
	if target == [32]byte{} || network == [32]byte{} || at.IsZero() || len(record) != publicProofSize || string(record[:len(publicationDomain)]) != publicationDomain {
		return Proof{}, errors.New("publication public proof input invalid")
	}
	raw := record[len(publicationDomain) : len(publicationDomain)+credentialSize]
	if binary.BigEndian.Uint16(raw[:2]) != 3 {
		return Proof{}, errors.New("publication Credential version invalid")
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
		return Proof{}, errors.New("publication Credential binding or capability invalid")
	}
	value.NotBefore, value.NotAfter = time.Unix(int64(before), 0).UTC(), time.Unix(int64(after), 0).UTC()
	if at.Before(value.NotBefore) || !at.Before(value.NotAfter) || !ed25519.Verify(ed25519.PublicKey(value.Authority[:]), raw[:credentialBodySize], raw[credentialBodySize:]) {
		return Proof{}, errors.New("publication Credential validity or signature invalid")
	}
	commitment := sha256.Sum256(record[:len(record)-ed25519.SignatureSize])
	if !ed25519.Verify(ed25519.PublicKey(value.Instance[:]), commitment[:], record[len(record)-ed25519.SignatureSize:]) {
		return Proof{}, errors.New("publication Instance proof invalid")
	}
	return Proof{delegation: value, digest: sha256.Sum256(record)}, nil
}
