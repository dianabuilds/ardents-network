package reachability_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
)

func TestDescriptorDraftMatchesIndependentCanonicalVector(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(3)
	length := int(binary.BigEndian.Uint16(raw[282:284]))
	record := bytes.Clone(raw[284 : 284+length])
	intro := reachability.Introduction{Revision: 9, Node: [32]byte{0x55}, Slot: [32]byte{0x66}, RecipientKey: [32]byte{0x77}, NotBefore: at, NotAfter: at.Add(600 * time.Second)}
	draft, err := reachability.PrepareDescriptor(record, target, network, profile, intro, at)
	if err != nil {
		t.Fatal(err)
	}
	instance := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x22}, 32))
	transcript := draft.Transcript()
	signature := ed25519.Sign(instance, transcript)
	if !bytes.Equal(signature, raw[len(raw)-64:]) {
		t.Fatal("producer transcript differs from independently encoded signature")
	}
	// Neither input nor returned codec bytes may mutate the prepared transcript.
	clear(record)
	clear(transcript)
	proof, err := draft.Complete(signature, at)
	if err != nil || !bytes.Equal(proof.Bytes(), raw) {
		t.Fatal("producer changed canonical independent Descriptor vector", err)
	}
	if _, err := draft.Complete(signature, at.Add(600*time.Second)); err == nil {
		t.Fatal("completion revived an expired draft")
	}
	signature[0] ^= 1
	if _, err := draft.Complete(signature, at); err == nil {
		t.Fatal("completion accepted a different signature")
	}
	if _, err := (reachability.Draft{}).Complete(make([]byte, 64), at); err == nil {
		t.Fatal("zero draft supplied a Descriptor")
	}
}

func TestDescriptorDraftRejectsUnboundedOrUnboundRecipient(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(3)
	length := int(binary.BigEndian.Uint16(raw[282:284]))
	record := raw[284 : 284+length]
	valid := reachability.Introduction{Revision: 9, Node: [32]byte{0x55}, Slot: [32]byte{0x66}, RecipientKey: [32]byte{0x77}, NotBefore: at, NotAfter: at.Add(600 * time.Second)}
	for _, change := range []func(*reachability.Introduction){
		func(v *reachability.Introduction) { v.Revision = 0 },
		func(v *reachability.Introduction) { v.Node = [32]byte{} },
		func(v *reachability.Introduction) { v.Slot = [32]byte{} },
		func(v *reachability.Introduction) { v.RecipientKey = [32]byte{} },
		func(v *reachability.Introduction) { v.NotBefore = at.Add(time.Nanosecond) },
		func(v *reachability.Introduction) { v.NotAfter = at.Add(600*time.Second + time.Nanosecond) },
		func(v *reachability.Introduction) { v.NotAfter = at.Add(601 * time.Second) },
		func(v *reachability.Introduction) { v.NotAfter = at },
		func(v *reachability.Introduction) { v.NotBefore = at.Add(-time.Second) },
	} {
		intro := valid
		change(&intro)
		if _, err := reachability.PrepareDescriptor(record, target, network, profile, intro, at); err == nil {
			t.Fatal("unbound or unbounded recipient accepted", intro)
		}
	}
	if _, err := reachability.PrepareDescriptor(record, target, network, [32]byte{}, valid, at); err == nil {
		t.Fatal("missing profile accepted")
	}
	readOnly, _, _, _, _ := signedDescriptor(2)
	length = int(binary.BigEndian.Uint16(readOnly[282:284]))
	if _, err := reachability.PrepareDescriptor(readOnly[284:284+length], target, network, profile, valid, at); err == nil {
		t.Fatal("Connect-only Credential supplied publication production")
	}
}
