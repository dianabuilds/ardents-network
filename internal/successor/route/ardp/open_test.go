package ardp

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"
)

func TestOpenIndependentCanonicalRecipientBytes(t *testing.T) {
	want, err := hex.DecodeString("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f200102030405060708070000000001020304")
	if err != nil {
		t.Fatal(err)
	}
	o := Open{RecipientDutyGeneration: 0x0102030405060708, Purpose: 7, Deadline: time.Unix(0x01020304, 0).UTC()}
	for i := range o.RecipientNodeID {
		o.RecipientNodeID[i] = byte(i + 1)
	}
	if body := EncodeOpen(o, false); !bytes.Equal(body, want) {
		t.Fatalf("recipient bytes changed: %x", body)
	}
	if body := EncodeOpen(o, true); !bytes.Equal(body, append(append([]byte(nil), want...), 0)) {
		t.Fatalf("Node restriction envelope changed: %x", body)
	}
	decoded, err := DecodeOpen(want)
	if err != nil || decoded != o {
		t.Fatalf("independent recipient decoding: %+v / %v", decoded, err)
	}
	// The vector is intentionally expired. Codec success grants no current
	// authority and cannot depend on the execution host's wall clock.
	for _, purpose := range []uint8{1, 3, 4, 5, 6} {
		body := append([]byte(nil), want...)
		body[40] = purpose
		decoded, err := DecodeOpen(body)
		if err != nil || decoded.Purpose != purpose || !bytes.Equal(EncodeOpen(decoded, false), body) {
			t.Fatal("selected grammar changed", purpose, err)
		}
	}
}

func TestOpenRejectsWrongShapeAndUnusableFields(t *testing.T) {
	valid := EncodeOpen(Open{RecipientNodeID: [32]byte{1}, RecipientDutyGeneration: 1, Purpose: 7, Deadline: time.Unix(1, 0)}, false)
	for _, body := range [][]byte{nil, valid[:48], append(append([]byte(nil), valid...), 0)} {
		if _, err := DecodeOpen(body); err == nil {
			t.Fatal("recipient description accepted wrong envelope", len(body))
		}
	}
	for _, field := range []string{"node", "duty", "purpose"} {
		body := append([]byte(nil), valid...)
		switch field {
		case "node":
			clear(body[:32])
		case "duty":
			clear(body[32:40])
		case "purpose":
			body[40] = 2
		}
		if _, err := DecodeOpen(body); err == nil {
			t.Fatal("unusable recipient description accepted", field)
		}
	}
}

func TestNodeOpenIndependentRestrictionEnvelopes(t *testing.T) {
	recipient, err := hex.DecodeString("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f200102030405060708010000000001020304")
	if err != nil {
		t.Fatal(err)
	}
	for _, restriction := range []ChildRestriction{OrdinaryChild, IssuerBootstrapChild} {
		body := append(append([]byte(nil), recipient...), byte(restriction))
		opened, err := DecodeNodeOpen(body)
		if err != nil || opened.Restriction != restriction || opened.Recipient.Purpose != 1 {
			t.Fatalf("Node envelope: %+v / %v", opened, err)
		}
		encoded, err := EncodeNodeOpen(opened)
		if err != nil || !bytes.Equal(encoded, body) {
			t.Fatalf("restriction bytes changed: %x / %v", encoded, err)
		}
		if _, err := DecodeOpen(body); err == nil {
			t.Fatal("Endpoint accepted an injected Node restriction")
		}
	}
	for _, body := range [][]byte{nil, recipient, append(append([]byte(nil), recipient...), 2), append(append([]byte(nil), recipient...), 255)} {
		if _, err := DecodeNodeOpen(body); err == nil {
			t.Fatal("retired or unknown Node envelope accepted", len(body))
		}
	}
	if raw, err := EncodeNodeOpen(NodeOpen{Restriction: ChildRestriction(2)}); err == nil || raw != nil {
		t.Fatal("unknown restriction emitted")
	}
}
