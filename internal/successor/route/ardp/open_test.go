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
	for _, purpose := range []uint8{4, 6} {
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
			body[40] = 1
		}
		if _, err := DecodeOpen(body); err == nil {
			t.Fatal("unusable recipient description accepted", field)
		}
	}
}
