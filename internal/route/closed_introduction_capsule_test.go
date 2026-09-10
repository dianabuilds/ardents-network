//go:build linux

package route

import (
	"bytes"
	"crypto/ecdh"
	"crypto/hpke"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"
)

type closedCapsuleTestRecipient struct{ private hpke.PrivateKey }

func (recipient closedCapsuleTestRecipient) OpenPrivateIntroduction(enc, info, aad, ciphertext []byte, _ time.Time) ([]byte, error) {
	receiver, err := hpke.NewRecipient(enc, recipient.private, hpke.HKDFSHA256(), hpke.AES128GCM(), info)
	if err != nil {
		return nil, err
	}
	return receiver.Open(aad, ciphertext)
}

func TestClosedIntroductionCapsuleCanonicalBytesAndAuthenticatedHeader(t *testing.T) {
	at := time.Unix(2_000_000_000, 0).UTC()
	repeated := func(value byte) [32]byte {
		var field [32]byte
		for index := range field {
			field[index] = value
		}
		return field
	}
	plaintext := ClosedIntroductionPlaintext{Network: repeated(1), Target: repeated(2), PublicationDigest: repeated(3), Revision: 1,
		RendezvousNode: repeated(4), RendezvousDutyGeneration: 2, JoinSecret: repeated(5), HandshakeContext: repeated(6), ProfileDigest: repeated(7), ConnectionNonce: repeated(8),
		AttachmentGeneration: 1, Deadline: at.Add(time.Second), InitiatorBinding: repeated(9), WorkSafetyNotAfter: 2_100_000_000, WorkSafetyMaximum: 2_200_000_000, NoNewRecoveryAfter: 2_000_000_000}
	raw, err := encodeClosedIntroductionPlaintext(plaintext)
	if err != nil || len(raw) != 344 {
		t.Fatalf("plaintext: %d %v", len(raw), err)
	}
	// Literal schema offsets independently pin nine 32-byte fields and all
	// seven unsigned big-endian integers, including the nonadjacent revision.
	for index, offset := range []int{0, 32, 64, 104, 144, 176, 208, 240, 288} {
		if !bytes.Equal(raw[offset:offset+32], bytes.Repeat([]byte{byte(index + 1)}, 32)) {
			t.Fatalf("field %d moved", index)
		}
	}
	for index, offset := range []int{96, 136, 272, 280, 320, 328, 336} {
		expected := []uint64{1, 2, 1, 2_000_000_001, 2_100_000_000, 2_200_000_000, 2_000_000_000}[index]
		if binary.BigEndian.Uint64(raw[offset:offset+8]) != expected {
			t.Fatalf("integer %d moved", index)
		}
	}
	if decoded, err := decodeClosedIntroductionPlaintext(raw); err != nil || decoded != plaintext {
		t.Fatalf("decode: %v", err)
	}
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	private, err := hpke.NewDHKEMPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	recipient := closedCapsuleTestRecipient{private: private}
	envelope := ClosedIntroductionCapsule{Slot: repeated(10), Revision: 1, Expiry: plaintext.Deadline, DeliveryNonce: repeated(11)}
	sealed, digest, err := SealClosedIntroduction(envelope, [32]byte(key.PublicKey().Bytes()), plaintext)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := EncodeClosedIntroductionSubmission(repeated(12), sealed)
	if err != nil || len(operation) != 4096 {
		t.Fatalf("operation: %v", err)
	}
	if operation[0] != 4 || !bytes.Equal(operation[1:33], bytes.Repeat([]byte{12}, 32)) || !bytes.Equal(operation[33:65], bytes.Repeat([]byte{10}, 32)) ||
		binary.BigEndian.Uint64(operation[65:73]) != 1 || binary.BigEndian.Uint64(operation[73:81]) != 2_000_000_001 ||
		!bytes.Equal(operation[81:113], bytes.Repeat([]byte{11}, 32)) || binary.BigEndian.Uint16(operation[145:147]) != 360 ||
		!bytes.Equal(operation[507:], make([]byte, 3589)) {
		t.Fatal("outer capsule framing changed")
	}
	nonce, decoded, err := DecodeClosedIntroductionSubmission(operation)
	if err != nil || nonce != repeated(12) {
		t.Fatalf("submission decode: %v", err)
	}
	opened, openedDigest, err := OpenClosedIntroduction(decoded, plaintext.ProfileDigest, recipient, at)
	if err != nil || opened != plaintext || openedDigest != digest || digest != sha256.Sum256(raw) {
		t.Fatalf("HPKE: %v", err)
	}
	// Independently use Go HPKE with the literal domain and exact header to
	// verify that both wrapper directions did not agree on a wrong transcript.
	info := append([]byte("ardents-introduction-capsule-v3\x00"), bytes.Repeat([]byte{7}, 32)...)
	receiver, err := hpke.NewRecipient(sealed.Encapsulation[:], private, hpke.HKDFSHA256(), hpke.AES128GCM(), info)
	if err != nil {
		t.Fatal(err)
	}
	independent, err := receiver.Open(append(info, operation[33:113]...), sealed.Ciphertext)
	if err != nil || !bytes.Equal(independent, raw) {
		t.Fatalf("literal HPKE transcript: %v", err)
	}
	for _, mutation := range []struct {
		name   string
		change func(*ClosedIntroductionCapsule)
	}{
		{"slot", func(v *ClosedIntroductionCapsule) { v.Slot[0] ^= 1 }},
		{"revision", func(v *ClosedIntroductionCapsule) { v.Revision++ }},
		{"expiry", func(v *ClosedIntroductionCapsule) { v.Expiry = v.Expiry.Add(time.Second) }},
		{"delivery nonce", func(v *ClosedIntroductionCapsule) { v.DeliveryNonce[0] ^= 1 }},
		{"encapsulation", func(v *ClosedIntroductionCapsule) { v.Encapsulation[0] ^= 1 }},
		{"ciphertext", func(v *ClosedIntroductionCapsule) { v.Ciphertext[0] ^= 1 }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			changed := sealed
			changed.Ciphertext = append([]byte(nil), sealed.Ciphertext...)
			mutation.change(&changed)
			if _, _, err := OpenClosedIntroduction(changed, plaintext.ProfileDigest, recipient, at); err == nil {
				t.Fatal("changed authenticated envelope accepted")
			}
		})
	}
	if _, _, err := OpenClosedIntroduction(sealed, repeated(13), recipient, at); err == nil {
		t.Fatal("foreign profile opened capsule")
	}
	if _, _, err := OpenClosedIntroduction(sealed, plaintext.ProfileDigest, recipient, sealed.Expiry); err == nil {
		t.Fatal("expired capsule opened")
	}
	for _, offset := range []int{0, 1, 145, 507, 4095} {
		changed := append([]byte(nil), operation...)
		if offset == 1 {
			clear(changed[1:33])
		} else {
			changed[offset] ^= 1
		}
		if _, _, err := DecodeClosedIntroductionSubmission(changed); err == nil {
			t.Fatalf("invalid canonical field/padding at%d accepted", offset)
		}
	}
	if _, _, err := DecodeClosedIntroductionSubmission(append(operation, operation...)); err == nil {
		t.Fatal("concatenated operations accepted")
	}
	for _, offset := range []int{280, 320, 328, 336} {
		changed := append([]byte(nil), raw...)
		changed[offset] |= 0x80
		if _, err := decodeClosedIntroductionPlaintext(changed); err == nil {
			t.Fatalf("signed bound overflow at%d", offset)
		}
	}
}
