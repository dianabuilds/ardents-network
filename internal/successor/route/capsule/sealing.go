package capsule

import (
	"crypto/ecdh"
	"crypto/hpke"
	"crypto/sha256"
	"errors"
)

// Seal encodes and encrypts one exact candidate request using the selected
// X25519/HKDF-SHA256/AES-128-GCM suite. It cannot attest recipient authority.
// The digest binds the exact plaintext to a later Attachment exporter.
func Seal(h Header, recipient [32]byte, request Request) (Envelope, [32]byte, error) {
	if !validHeader(h) || recipient == [32]byte{} || request.Revision != h.Revision || request.Deadline != h.Expiry {
		return Envelope{}, [32]byte{}, errors.New("introduction sealing binding invalid")
	}
	raw, err := encodeRequest(request)
	if err != nil {
		return Envelope{}, [32]byte{}, err
	}
	defer clear(raw)
	public, err := ecdh.X25519().NewPublicKey(recipient[:])
	if err != nil {
		return Envelope{}, [32]byte{}, err
	}
	key, err := hpke.NewDHKEMPublicKey(public)
	if err != nil {
		return Envelope{}, [32]byte{}, err
	}
	e := Envelope{header: h}
	enc, sender, err := hpke.NewSender(key, hpke.HKDFSHA256(), hpke.AES128GCM(), Info(request.ProfileDigest))
	if err != nil {
		return Envelope{}, [32]byte{}, err
	}
	ciphertext, err := sender.Seal(e.AssociatedData(request.ProfileDigest), raw)
	if err != nil || len(enc) != 32 || len(ciphertext) != 360 {
		return Envelope{}, [32]byte{}, errors.Join(errors.New("introduction sealing failed"), err)
	}
	copy(e.encapsulation[:], enc)
	copy(e.ciphertext[:], ciphertext)
	return e, sha256.Sum256(raw), nil
}
