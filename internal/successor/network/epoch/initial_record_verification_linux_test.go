package epoch

import (
	"bytes"
	"crypto/ed25519"
	"testing"
	"time"
)

func TestVerifyInitialClosedRecordExactPreparation(t *testing.T) {
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{73}, 32))
	defer clear(key)
	input := InitialClosedRecord{NetworkID: [32]byte{1}, NodeID: [32]byte{2}, ValidFrom: time.Unix(2000000000, 0).UTC(), ValidUntil: time.Unix(2000000600, 0).UTC(), Family: "family", Endpoint: "127.0.0.1:4401", Carrier: CarrierClosedTCP, Capacity: 1, PublicKey: key.Public().(ed25519.PublicKey)}
	prepared, err := PrepareInitialClosedRecord(input)
	if err != nil {
		t.Fatal(err)
	}
	signed := append(bytes.Clone(prepared), ed25519.Sign(key, prepared)...)
	facts, err := VerifyInitialClosedRecord(prepared, signed)
	if err != nil || facts.NodeID != input.NodeID || facts.NetworkID != input.NetworkID || !facts.ValidFrom.Equal(input.ValidFrom) || !facts.ValidUntil.Equal(input.ValidUntil) || !bytes.Equal(facts.PublicKey, input.PublicKey) {
		t.Fatal("verified facts", err)
	}
	facts.PublicKey[0] ^= 1
	if !bytes.Equal(input.PublicKey, key[32:]) {
		t.Fatal("returned facts alias input")
	}
	for _, fault := range []string{"signature", "trailing", "prepared", "generation", "version", "capability", "truncated"} {
		t.Run(fault, func(t *testing.T) {
			p, r := bytes.Clone(prepared), bytes.Clone(signed)
			switch fault {
			case "signature":
				r[len(r)-1] ^= 1
			case "trailing":
				r = append(r, 0)
			case "prepared":
				p[5] ^= 1
			case "generation":
				r[76] = 2
				copy(r[len(r)-64:], ed25519.Sign(key, r[:len(r)-64]))
			case "version":
				r[4] = 1
			case "capability":
				r[94+len(input.Family)] = 1
				copy(r[len(r)-64:], ed25519.Sign(key, r[:len(r)-64]))
			case "truncated":
				r = r[:30]
			}
			if _, err := VerifyInitialClosedRecord(p, r); err == nil {
				t.Fatal("invalid initial record accepted")
			}
			if _, err := VerifyInitialClosedRecord(prepared, signed); err != nil {
				t.Fatal("healthy control", err)
			}
		})
	}
}
