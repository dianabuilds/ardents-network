package publication_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

// Independent signed input, without an accepting Publisher or readiness ACK.
func publicRecord(caps uint32) ([]byte, [32]byte, [32]byte, time.Time) {
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, 32))
	instance := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x22}, 32))
	pub := authority.Public().(ed25519.PublicKey)
	target := sha256.Sum256(append([]byte("ardents-service-target-v3\x00"), pub...))
	network := [32]byte{0x33}
	at := time.Unix(1900000000, 0).UTC()
	credential := []byte{0, 3}
	credential = append(credential, pub...)
	credential = append(credential, target[:]...)
	credential = append(credential, instance.Public().(ed25519.PublicKey)...)
	for _, number := range []uint64{7, uint64(at.Unix()), uint64(at.Add(time.Hour).Unix())} {
		credential = binary.BigEndian.AppendUint64(credential, number)
	}
	credential = append(credential, network[:]...)
	credential = binary.BigEndian.AppendUint32(credential, caps)
	credential = append(credential, ed25519.Sign(authority, credential)...)
	record := append([]byte("ardents-service-publication-v3\x00"), credential...)
	record = append(record, bytes.Repeat([]byte{0x44}, 32)...)
	commitment := sha256.Sum256(record)
	record = append(record, ed25519.Sign(instance, commitment[:])...)
	return record, target, network, at
}

func TestPublicProofPublishRequiresDelegationBeyondConnect(t *testing.T) {
	for _, caps := range []uint32{0, 1, 2, 3} {
		raw, target, network, at := publicRecord(caps)
		_, readErr := publication.Verify(raw, target, network, at)
		_, publishErr := publication.VerifyPublish(raw, target, network, at)
		if (readErr == nil) != (caps&2 != 0) {
			t.Fatalf("read capability mismatch for %d: %v", caps, readErr)
		}
		if (publishErr == nil) != (caps == 3) {
			t.Fatalf("publish capability mismatch for %d: %v", caps, publishErr)
		}
	}
}

func TestPublicProofBindingsAndCopiedFacts(t *testing.T) {
	raw, target, network, at := publicRecord(3)
	proof, err := publication.VerifyPublish(raw, target, network, at)
	if err != nil {
		t.Fatal(err)
	}
	expected := sha256.Sum256(raw)
	if proof.Digest() != expected || proof.Delegation().Generation != 7 || proof.Delegation().Target != target {
		t.Fatal("signed public facts changed")
	}
	copyOut := proof.Delegation()
	copyOut.Target[0] ^= 1
	copyOut.Generation++
	raw[0] ^= 1
	if proof.Digest() != expected || proof.Delegation().Target != target || proof.Delegation().Generation != 7 {
		t.Fatal("caller mutated retained public proof")
	}
	raw, target, network, at = publicRecord(3)
	for _, test := range []struct {
		name            string
		target, network [32]byte
		at              time.Time
	}{
		{"wrong-target", [32]byte{1}, network, at},
		{"wrong-network", target, [32]byte{1}, at},
		{"before", target, network, at.Add(-time.Nanosecond)},
		{"expiry", target, network, at.Add(time.Hour)},
		{"zero-time", target, network, time.Time{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := publication.VerifyPublish(raw, test.target, test.network, test.at); err == nil {
				t.Fatal("invalid proof context accepted")
			}
		})
	}
	for _, offset := range []int{0, len("ardents-service-publication-v3\x00"), len(raw) - 65, len(raw) - 1} {
		broken := bytes.Clone(raw)
		broken[offset] ^= 1
		if _, err := publication.VerifyPublish(broken, target, network, at); err == nil {
			t.Fatalf("altered signed byte %d accepted", offset)
		}
	}
	for _, broken := range [][]byte{nil, raw[:len(raw)-1], append(bytes.Clone(raw), 0)} {
		if _, err := publication.VerifyPublish(broken, target, network, at); err == nil {
			t.Fatal("wrong public record length accepted")
		}
	}
}
