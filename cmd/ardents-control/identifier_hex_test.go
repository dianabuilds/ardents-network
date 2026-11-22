package main

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestDecodeHex32RequiresCanonicalLowercaseShape(t *testing.T) {
	canonical := strings.Repeat("0a", 32)
	value, ok := decodeHex32(canonical)
	if !ok || hex.EncodeToString(value[:]) != canonical {
		t.Fatal("canonical 32-byte hex refused")
	}
	for _, raw := range []string{
		"",
		strings.Repeat("0a", 31),
		strings.Repeat("0a", 33),
		strings.Repeat("0A", 32),
		strings.Repeat("0a", 31) + "0",
		"zz" + strings.Repeat("0a", 31),
	} {
		if _, ok := decodeHex32(raw); ok {
			t.Fatal("non-canonical hex accepted", raw)
		}
	}
}

// The portable closed-profile identifier reader keeps its own error text and
// zero-value policy; only the canonical hex shape is shared (audit E1-4).
func TestDecodeIdentifierKeepsClosedProfileContract(t *testing.T) {
	value, err := decodeIdentifier(strings.Repeat("00", 32))
	if err != nil || value != [32]byte{} {
		t.Fatal("closed-profile zero identifier policy changed")
	}
	if _, err := decodeIdentifier(strings.Repeat("0A", 32)); err == nil || err.Error() != "alpha control identifier is invalid" {
		t.Fatal("closed-profile identifier error contract changed")
	}
}
