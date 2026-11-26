package canonicaldigest

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestDecodeAdmitsOnlyCanonicalLowercaseHex32(t *testing.T) {
	canonical := strings.Repeat("0a", 32)
	value, ok := Decode(canonical)
	if !ok || hex.EncodeToString(value[:]) != canonical {
		t.Fatal("canonical 32-byte hex refused")
	}
	var wanted [32]byte
	for i := range wanted {
		wanted[i] = 0x0a
	}
	if value != wanted {
		t.Fatal("decoded bytes differ from the canonical input")
	}
	if !Valid(canonical) {
		t.Fatal("validation-only projection refused the canonical input")
	}
	zero := strings.Repeat("00", 32)
	if zeroValue, ok := Decode(zero); !ok || zeroValue != [32]byte{} {
		t.Fatal("the zero digest is canonical and must decode to the zero value")
	}
	for _, raw := range []string{
		"",
		strings.Repeat("0a", 31),
		strings.Repeat("0a", 33),
		strings.Repeat("0A", 32),
		strings.Repeat("0a", 31) + "0",
		strings.Repeat("0a", 31) + "0A",
		"zz" + strings.Repeat("0a", 31),
		strings.Repeat("0a", 31) + "g0",
		"0x" + strings.Repeat("0a", 31),
		" " + strings.Repeat("0a", 31) + "0",
		strings.Repeat("0a", 32) + "\n",
		strings.Repeat("0a", 64),
	} {
		value, ok := Decode(raw)
		if ok {
			t.Fatal("non-canonical hex accepted", raw)
		}
		if value != [32]byte{} {
			t.Fatal("refused decode must return the zero value", raw)
		}
		if Valid(raw) {
			t.Fatal("validation-only projection accepted non-canonical hex", raw)
		}
	}
}

func TestValidMatchesDecodeAdmission(t *testing.T) {
	for _, raw := range []string{
		"",
		"0",
		"00",
		strings.Repeat("00", 32),
		strings.Repeat("ff", 32),
		strings.Repeat("FF", 32),
		strings.Repeat("aB", 32),
		strings.Repeat("0123456789abcdef", 4),
		strings.Repeat("0a", 31) + "0",
		strings.Repeat("0a", 33),
		strings.Repeat("0a", 31) + "zz",
		"0x" + strings.Repeat("0a", 31),
	} {
		_, ok := Decode(raw)
		if Valid(raw) != ok {
			t.Fatalf("Valid and Decode disagree on %q", raw)
		}
	}
}

func TestDecodeRefusesOversizedInputWithoutAllocating(t *testing.T) {
	oversized := strings.Repeat("a", 1<<20)
	allocations := testing.AllocsPerRun(10, func() {
		value, ok := Decode(oversized)
		if ok || value != [32]byte{} {
			t.Fatal("oversized digest was accepted")
		}
	})
	if allocations != 0 {
		t.Fatalf("oversized digest allocated %.0f times per call", allocations)
	}
}
