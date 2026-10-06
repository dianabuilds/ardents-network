package reachability_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
)

// Construct the accepted transcripts directly, never through production codecs
// or old domain helpers. This is public signed input, not successful readiness.
func signedDescriptor(caps uint32) ([]byte, [32]byte, [32]byte, [32]byte, time.Time) {
	return signedDescriptorForAuthority(caps, 0x11)
}

func signedDescriptorForAuthority(caps uint32, seed byte) ([]byte, [32]byte, [32]byte, [32]byte, time.Time) {
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, 32))
	instance := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x22}, 32))
	public := authority.Public().(ed25519.PublicKey)
	target := sha256.Sum256(append([]byte("ardents-service-target-v3\x00"), public...))
	network, profile := [32]byte{0x33}, [32]byte{0x44}
	at := time.Unix(1900000000, 0).UTC()
	credential := []byte{0, 3}
	credential = append(credential, public...)
	credential = append(credential, target[:]...)
	credential = append(credential, instance.Public().(ed25519.PublicKey)...)
	for _, number := range []uint64{7, uint64(at.Unix()), uint64(at.Add(time.Hour).Unix())} {
		credential = binary.BigEndian.AppendUint64(credential, number)
	}
	credential = append(credential, network[:]...)
	credential = binary.BigEndian.AppendUint32(credential, caps)
	credential = append(credential, ed25519.Sign(authority, credential)...)
	record := append([]byte("ardents-service-publication-v3\x00"), credential...)
	record = append(record, make([]byte, 32)...)
	commitment := sha256.Sum256(record)
	record = append(record, ed25519.Sign(instance, commitment[:])...)
	digest := sha256.Sum256(record)
	recipient := binary.BigEndian.AppendUint64(nil, 9)
	for _, key := range [][32]byte{{0x55}, {0x66}, {0x77}} {
		recipient = append(recipient, key[:]...)
	}
	for _, seconds := range []uint64{uint64(at.Unix()), uint64(at.Add(600 * time.Second).Unix())} {
		recipient = binary.BigEndian.AppendUint64(recipient, seconds)
	}
	transcript := []byte("ardents-private-reachability-v3\x00")
	transcript = append(transcript, network[:]...)
	transcript = append(transcript, profile[:]...)
	transcript = append(transcript, digest[:]...)
	transcript = append(transcript, recipient...)
	body := []byte{0, 3}
	body = append(body, network[:]...)
	body = append(body, target[:]...)
	body = append(body, public...)
	body = append(body, digest[:]...)
	body = append(body, profile[:]...)
	body = append(body, recipient...)
	body = binary.BigEndian.AppendUint16(body, uint16(len(record)))
	body = append(body, record...)
	return append(body, ed25519.Sign(instance, transcript)...), target, network, profile, at
}

func TestDescriptorPublicationRejectsConnectOnlySignedInput(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(2)
	if _, err := reachability.Verify(raw, target, network, profile, at); err != nil {
		t.Fatal("read control refused", err)
	}
	if _, err := reachability.VerifyPublish(raw, network, profile, at); err == nil {
		t.Fatal("publication accepted without delegated Publish")
	}
	raw, target, network, profile, at = signedDescriptor(3)
	proof, err := reachability.VerifyPublish(raw, network, profile, at)
	if err != nil || proof.Target != target || proof.Introduction.Revision != 9 || proof.Introduction.Node != [32]byte{0x55} {
		t.Fatal("valid independently signed proof differs", proof, err)
	}
	expected := sha256.Sum256(raw)
	raw[0] ^= 1
	copied := proof.Bytes()
	copied[0] ^= 1
	if proof.Digest() != expected || sha256.Sum256(proof.Bytes()) != expected {
		t.Fatal("caller mutated verified Descriptor bytes")
	}
}

func TestDescriptorRefusesWrongBindingsTimesAndSignatures(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(3)
	for _, test := range []struct {
		name                     string
		target, network, profile [32]byte
		at                       time.Time
	}{
		{"target", [32]byte{1}, network, profile, at},
		{"network", target, [32]byte{1}, profile, at},
		{"profile", target, network, [32]byte{1}, at},
		{"before", target, network, profile, at.Add(-time.Nanosecond)},
		{"expiry", target, network, profile, at.Add(600 * time.Second)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := reachability.Verify(raw, test.target, test.network, test.profile, test.at); err == nil {
				t.Fatal("wrong lookup context accepted")
			}
		})
	}
	for _, offset := range []int{0, 2, 34, 66, 98, 130, 162, 170, 202, 234, 266, 274, 282, 284, len(raw) - 1} {
		broken := bytes.Clone(raw)
		broken[offset] ^= 1
		if _, err := reachability.VerifyPublish(broken, network, profile, at); err == nil {
			t.Fatalf("altered signed field at %d accepted", offset)
		}
	}
	for _, broken := range [][]byte{nil, raw[:283], raw[:len(raw)-1], append(bytes.Clone(raw), 0), make([]byte, 15001)} {
		if _, err := reachability.VerifyPublish(broken, network, profile, at); err == nil {
			t.Fatal("wrong length accepted")
		}
	}
	malicious := bytes.Clone(raw)
	malicious[282], malicious[283] = 255, 255
	if _, err := reachability.VerifyPublish(malicious, network, profile, at); err == nil {
		t.Fatal("hostile embedded length accepted")
	}
}

func TestDescriptorCurrentUsesRetainedSignedInterval(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(3)
	proof, err := reachability.Verify(raw, target, network, profile, at)
	if err != nil {
		t.Fatal(err)
	}
	end := proof.Introduction.NotAfter
	// The projection and returned byte copy cannot extend verified validity.
	proof.Introduction.NotBefore = at.Add(-time.Hour)
	proof.Introduction.NotAfter = end.Add(time.Hour)
	copy := proof.Bytes()
	binary.BigEndian.PutUint64(copy[274:282], uint64(end.Add(time.Hour).Unix()))
	for _, point := range []time.Time{at, end.Add(-time.Nanosecond)} {
		if err := proof.Current(point); err != nil {
			t.Fatal("valid signed interval refused", err)
		}
	}
	for _, point := range []time.Time{{}, at.Add(-time.Nanosecond), end, end.Add(time.Nanosecond)} {
		if err := proof.Current(point); err == nil {
			t.Fatal("unsigned or expired interval accepted", point)
		}
	}
	if err := (reachability.Descriptor{}).Current(at); err == nil {
		t.Fatal("absent verified proof accepted")
	}
}

// Re-sign independently modified public fixture bytes. This never represents a
// successful live Publisher, recipient or fake storage acknowledgement.
func resignDescriptor(raw []byte, change func([]byte)) []byte {
	body := bytes.Clone(raw)
	change(body)
	start := 284
	end := len(body) - 64
	credentialStart := start + len("ardents-service-publication-v3\x00")
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, 32))
	instance := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x22}, 32))
	copy(body[credentialStart+158:credentialStart+222], ed25519.Sign(authority, body[credentialStart:credentialStart+158]))
	commitment := sha256.Sum256(body[start : end-64])
	copy(body[end-64:end], ed25519.Sign(instance, commitment[:]))
	digest := sha256.Sum256(body[start:end])
	copy(body[98:130], digest[:])
	transcript := []byte("ardents-private-reachability-v3\x00")
	transcript = append(transcript, body[2:34]...)
	transcript = append(transcript, body[130:162]...)
	transcript = append(transcript, body[98:130]...)
	transcript = append(transcript, body[162:282]...)
	copy(body[end:], ed25519.Sign(instance, transcript))
	return body
}
