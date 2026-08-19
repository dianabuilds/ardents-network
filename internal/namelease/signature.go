package namelease

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

// Signature errors. Unexported; tests in the same package compare with
// errSignature* directly.
var (
	errSignatureMissing      = errors.New("name record signature is missing")
	errSignatureWrongKey     = errors.New("name record signature does not match the supplied public key")
	errSignatureBadCanonical = errors.New("name record signature payload is malformed")
)

// signedPayload returns the canonical byte representation of the
// immutable fields of a Record. The Authority field is hashed but the
// public-key bytes themselves are not signed into the payload: a
// verifier must already know which public key the Record is bound to
// (the Authority field carries its identifier). The payload is what
// the feasibility tracer currently covers: name, generation, revision, state,
// authority, target, parent.
func signedPayload(r Record) []byte {
	h := sha256.New()
	writeStringField(h, r.Name)
	writeUint64Field(h, r.Generation)
	writeUint64Field(h, r.Revision)
	writeStringField(h, r.State)
	writeStringField(h, r.Authority)
	writeStringField(h, r.Target)
	writeStringField(h, r.ParentName)
	// Domain separator so a signature over a Name Record is never
	// confused with any other use of the same key.
	const domainSep = "ardents-h3-name-record-v1\x00"
	h.Write([]byte(domainSep))
	return h.Sum(nil)
}

// sign signs the canonical Record payload with the supplied Ed25519
// private key and stores the signature in
// r.Signature. The Caller is responsible for ensuring the matching
// public key is what r.Authority is bound to.
func (r *Record) sign(priv ed25519.PrivateKey) error {
	if priv == nil {
		return errSignatureBadCanonical
	}
	sig, err := priv.Sign(nil, signedPayload(*r), &ed25519.Options{})
	if err != nil {
		return err
	}
	r.Signature = sig
	return nil
}

// verify returns nil iff r.Signature is a valid Ed25519 signature by
// pub over the canonical Record payload. The check is replay-safe
// only when combined with the generation/revision/parent/epoch
// invariants enforced at Apply time. R-044 remains open, so this helper
// demonstrates feasibility rather than selecting the Stage 6 suite.
func (r Record) verify(pub ed25519.PublicKey) error {
	if len(r.Signature) == 0 {
		return errSignatureMissing
	}
	if len(pub) != ed25519.PublicKeySize {
		return errSignatureWrongKey
	}
	if !ed25519.Verify(pub, signedPayload(r), r.Signature) {
		return errSignatureWrongKey
	}
	return nil
}

func writeStringField(h interface{ Write([]byte) (int, error) }, s string) {
	var lenBuf [8]byte
	binary.BigEndian.PutUint64(lenBuf[:], uint64(len(s)))
	h.Write(lenBuf[:])
	h.Write([]byte(s))
}

func writeUint64Field(h interface{ Write([]byte) (int, error) }, v uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	h.Write(buf[:])
}
