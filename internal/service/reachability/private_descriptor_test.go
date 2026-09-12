//go:build linux

package reachability_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func TestPrivateDescriptorBindsExactPublicationProfileAndRecipient(t *testing.T) {
	fixture := newDescriptorFixture(t)
	intro := reachability.PrivateIntroduction{Revision: 1, NodeID: [32]byte{41}, Slot: [32]byte{42}, RecipientKey: [32]byte{43},
		NotBefore: fixture.now, NotAfter: fixture.now.Add(30 * time.Second)}
	profile := [32]byte{44}
	raw, issued, err := reachability.IssuePrivate(reachability.PrivateIssueInput{
		Current: fixture.current, ProfileDigest: profile, Introduction: intro, InstanceSigner: fixture.instancePrivate,
	})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := reachability.VerifyPrivate(raw, fixture.current.Credential.Target, fixture.network, profile, fixture.now)
	if err != nil || verified.Descriptor.Version != 3 || verified.Descriptor.Private != intro || !bytes.Equal(verified.Current.Record, fixture.current.Record) {
		t.Fatalf("private Descriptor verification: %+v, %v", verified, err)
	}
	// Independent transcript assembly verifies the selected direct Ed25519
	// signature, not a mutually mistaken encoder/verifier round trip.
	transcript := []byte("ardents-private-reachability-v3\x00")
	transcript = append(transcript, fixture.network[:]...)
	transcript = append(transcript, profile[:]...)
	digest := sha256.Sum256(fixture.current.Record)
	transcript = append(transcript, digest[:]...)
	transcript = binary.BigEndian.AppendUint64(transcript, intro.Revision)
	for _, field := range [][32]byte{intro.NodeID, intro.Slot, intro.RecipientKey} {
		transcript = append(transcript, field[:]...)
	}
	transcript = binary.BigEndian.AppendUint64(transcript, uint64(intro.NotBefore.Unix()))
	transcript = binary.BigEndian.AppendUint64(transcript, uint64(intro.NotAfter.Unix()))
	if !ed25519.Verify(fixture.instancePrivate.Public().(ed25519.PublicKey), transcript, issued.Signature[:]) {
		t.Fatal("Instance signature differs from selected transcript")
	}
	if _, err := reachability.Verify(raw, fixture.current.Credential.Target, fixture.network, fixture.now); err == nil {
		t.Fatal("legacy verifier admitted private Descriptor without a profile")
	}
	if _, err := reachability.VerifyPrivate(raw, fixture.current.Credential.Target, fixture.network, [32]byte{45}, fixture.now); err == nil {
		t.Fatal("private Descriptor accepted a different current profile")
	}
	for offset := range raw {
		changed := bytes.Clone(raw)
		changed[offset] ^= 1
		if _, err := reachability.VerifyPrivate(changed, fixture.current.Credential.Target, fixture.network, profile, fixture.now); err == nil {
			t.Fatalf("altered byte %d accepted", offset)
		}
	}
	for _, invalid := range [][]byte{raw[:len(raw)-1], append(bytes.Clone(raw), 0), make([]byte, reachability.MaximumPrivateDescriptorSize+1)} {
		if _, err := reachability.VerifyPrivate(invalid, fixture.current.Credential.Target, fixture.network, profile, fixture.now); err == nil {
			t.Fatal("invalid Descriptor length accepted")
		}
	}
	if _, err := reachability.VerifyPrivate(raw, fixture.current.Credential.Target, fixture.network, profile, intro.NotAfter); err == nil {
		t.Fatal("expired private slot accepted")
	}
	if _, err := reachability.VerifyPrivate(raw, fixture.current.Credential.Target, fixture.network, profile, intro.NotBefore.Add(-time.Nanosecond)); err == nil {
		t.Fatal("private slot accepted before its signed start")
	}
}

func TestPrivateDescriptorRejectsMixedOrExtendedAuthority(t *testing.T) {
	fixture := newDescriptorFixture(t)
	intro := reachability.PrivateIntroduction{Revision: 1, NodeID: [32]byte{41}, Slot: [32]byte{42}, RecipientKey: [32]byte{43},
		NotBefore: fixture.now, NotAfter: fixture.now.Add(30 * time.Second)}
	for name, mutate := range map[string]func(*reachability.PrivateIssueInput){
		"missing profile":   func(value *reachability.PrivateIssueInput) { value.ProfileDigest = [32]byte{} },
		"zero revision":     func(value *reachability.PrivateIssueInput) { value.Introduction.Revision = 0 },
		"missing slot":      func(value *reachability.PrivateIssueInput) { value.Introduction.Slot = [32]byte{} },
		"missing recipient": func(value *reachability.PrivateIssueInput) { value.Introduction.RecipientKey = [32]byte{} },
		"fractional lifetime": func(value *reachability.PrivateIssueInput) {
			value.Introduction.NotAfter = value.Introduction.NotAfter.Add(time.Nanosecond)
		},
		"past Credential": func(value *reachability.PrivateIssueInput) {
			value.Introduction.NotAfter = time.Unix(fixture.current.Credential.NotAfter+1, 0)
		},
		"before Credential": func(value *reachability.PrivateIssueInput) {
			value.Introduction.NotBefore = time.Unix(fixture.current.Credential.NotBefore-1, 0)
		},
		"wrong signer": func(value *reachability.PrivateIssueInput) {
			value.InstanceSigner = ed25519.NewKeyFromSeed(make([]byte, 32))
		},
		"forged current": func(value *reachability.PrivateIssueInput) {
			value.Current.Record = bytes.Clone(value.Current.Record)
			value.Current.Record[20] ^= 1
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := reachability.PrivateIssueInput{Current: fixture.current, ProfileDigest: [32]byte{44}, Introduction: intro, InstanceSigner: fixture.instancePrivate}
			mutate(&input)
			if _, _, err := reachability.IssuePrivate(input); err == nil {
				t.Fatal("invalid private issuance accepted")
			}
		})
	}
}
