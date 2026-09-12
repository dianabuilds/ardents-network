//go:build linux

package route

import (
	"crypto/ecdh"
	"crypto/hpke"
	"crypto/sha256"
	"errors"
	"time"
)

const closedIntroductionInfo = "ardents-introduction-capsule-v3\x00"

// SealClosedIntroduction encrypts the exact fixed recipient-only tuple with
// the selected Go HPKE suite. The digest binds the later Attachment exporter.
func SealClosedIntroduction(input ClosedIntroductionCapsule, recipient [32]byte, plaintext ClosedIntroductionPlaintext) (ClosedIntroductionCapsule, [32]byte, error) {
	if !validClosedIntroductionHeader(input) || input.Encapsulation != [32]byte{} || len(input.Ciphertext) != 0 || recipient == [32]byte{} ||
		plaintext.Revision != input.Revision || !plaintext.Deadline.Equal(input.Expiry) {
		return ClosedIntroductionCapsule{}, [32]byte{}, errors.New("closed Introduction sealing binding invalid")
	}
	raw, err := encodeClosedIntroductionPlaintext(plaintext)
	if err != nil {
		return ClosedIntroductionCapsule{}, [32]byte{}, err
	}
	defer clear(raw)
	public, err := ecdh.X25519().NewPublicKey(recipient[:])
	if err != nil {
		return ClosedIntroductionCapsule{}, [32]byte{}, err
	}
	key, err := hpke.NewDHKEMPublicKey(public)
	if err != nil {
		return ClosedIntroductionCapsule{}, [32]byte{}, err
	}
	info := closedIntroductionDomain(plaintext.ProfileDigest)
	enc, sender, err := hpke.NewSender(key, hpke.HKDFSHA256(), hpke.AES128GCM(), info)
	if err != nil {
		return ClosedIntroductionCapsule{}, [32]byte{}, err
	}
	ciphertext, err := sender.Seal(append(info, closedIntroductionHeader(input)...), raw)
	if err != nil || len(enc) != 32 || len(ciphertext) != closedIntroductionCiphertextSize {
		return ClosedIntroductionCapsule{}, [32]byte{}, errors.Join(errors.New("closed Introduction sealing failed"), err)
	}
	copy(input.Encapsulation[:], enc)
	input.Ciphertext = ciphertext
	return input, sha256.Sum256(raw), nil
}

// OpenClosedIntroduction authenticates the profile and entire visible header
// before decoding private facts. Endpoint must independently check publication,
// registration, replay, live local authority and Rendezvous eligibility.
func OpenClosedIntroduction(input ClosedIntroductionCapsule, profile [32]byte, recipient ClosedIntroductionRecipient, at time.Time) (ClosedIntroductionPlaintext, [32]byte, error) {
	if !validClosedIntroductionHeader(input) || input.Encapsulation == [32]byte{} || len(input.Ciphertext) != closedIntroductionCiphertextSize ||
		profile == [32]byte{} || recipient == nil || at.IsZero() || !at.Before(input.Expiry) {
		return ClosedIntroductionPlaintext{}, [32]byte{}, errors.New("closed Introduction recipient input invalid")
	}
	info := closedIntroductionDomain(profile)
	raw, err := recipient.OpenPrivateIntroduction(input.Encapsulation[:], info, append(append([]byte(nil), info...), closedIntroductionHeader(input)...), input.Ciphertext, at)
	if err != nil {
		return ClosedIntroductionPlaintext{}, [32]byte{}, err
	}
	defer clear(raw)
	plaintext, err := decodeClosedIntroductionPlaintext(raw)
	if err != nil || plaintext.ProfileDigest != profile || plaintext.Revision != input.Revision || !plaintext.Deadline.Equal(input.Expiry) {
		return ClosedIntroductionPlaintext{}, [32]byte{}, errors.Join(errors.New("closed Introduction plaintext binding invalid"), err)
	}
	return plaintext, sha256.Sum256(raw), nil
}

func closedIntroductionDomain(profile [32]byte) []byte {
	return append([]byte(closedIntroductionInfo), profile[:]...)
}
