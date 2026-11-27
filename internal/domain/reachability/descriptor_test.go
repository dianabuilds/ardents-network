package reachability_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/reachability"
	"github.com/dianabuilds/ardents-network/tests/descriptorfixture"
)

func TestDescriptorPublicationRejectsConnectOnlySignedInput(t *testing.T) {
	raw, target, network, profile, at := descriptorfixture.Fixed(2, 0x11)
	if _, err := reachability.Verify(raw, target, network, profile, at); err != nil {
		t.Fatal("read control refused", err)
	}
	if _, err := reachability.VerifyPublish(raw, network, profile, at); err == nil {
		t.Fatal("publication accepted without delegated Publish")
	}
	raw, target, network, profile, at = descriptorfixture.Fixed(3, 0x11)
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
	raw, target, network, profile, at := descriptorfixture.Fixed(3, 0x11)
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
	raw, target, network, profile, at := descriptorfixture.Fixed(3, 0x11)
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
