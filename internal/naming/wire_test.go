package naming

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// TestWire_RoundTrip encodes and decodes canonical names and expects
// the result to be byte-identical to the input, per R-041
// length-prefixed wire encoding with frozen schema_version=1.
func TestWire_RoundTrip(t *testing.T) {
	t.Parallel()
	names := []string{
		"alice",
		"blog.alice",
		"blog-42.example-node",
		"a.b.c.d.e",
		"1alpha-2beta-3gamma.example",
	}
	for _, in := range names {
		name, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		wire, err := EncodeWire(name)
		if err != nil {
			t.Fatalf("EncodeWire(%q): %v", in, err)
		}
		got, err := DecodeWire(wire)
		if err != nil {
			t.Fatalf("DecodeWire(%q): %v", in, err)
		}
		if got != name {
			t.Errorf("round-trip %q: got %q", in, got)
		}
	}
}

// TestWire_SchemaVersionPrefixIsBigEndianTwoBytes per R-041 and
// ADR-0013: the first two bytes of every encoded name are the
// big-endian uint16 schema_version (value 1).
func TestWire_SchemaVersionPrefixIsBigEndianTwoBytes(t *testing.T) {
	t.Parallel()
	wire, err := EncodeWire(MustParse(t, "alice"))
	if err != nil {
		t.Fatalf("EncodeWire: %v", err)
	}
	if len(wire) < 2 {
		t.Fatalf("wire too short: %d bytes", len(wire))
	}
	got := binary.BigEndian.Uint16(wire[:2])
	if got != SchemaVersion {
		t.Errorf("schema_version = %d, want %d", got, SchemaVersion)
	}
	if SchemaVersion != 1 {
		t.Errorf("frozen SchemaVersion = %d, want 1 (R-041)", SchemaVersion)
	}
}

// TestWire_LabelsAreLeafToRoot per R-041: labels are serialized
// from leaf to root, parent on the right.
func TestWire_LabelsAreLeafToRoot(t *testing.T) {
	t.Parallel()
	wire, err := EncodeWire(MustParse(t, "blog.example"))
	if err != nil {
		t.Fatalf("EncodeWire: %v", err)
	}
	// expected: 0x00 0x01 || 0x04 'b' 'l' 'o' 'g' || 0x07 'e' 'x' 'a' 'm' 'p' 'l' 'e'
	want := []byte{
		0x00, 0x01,
		0x04, 'b', 'l', 'o', 'g',
		0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
	}
	if !bytes.Equal(wire, want) {
		t.Errorf("wire bytes = %x, want %x", wire, want)
	}
}

// TestWire_RejectsUnknownSchemaVersion per R-041: a wire with a
// different schema_version is rejected.
func TestWire_RejectsUnknownSchemaVersion(t *testing.T) {
	t.Parallel()
	bad := []byte{0x00, 0x02, 0x05, 'h', 'e', 'l', 'l', 'o'}
	_, err := DecodeWire(bad)
	if !errors.Is(err, errWireBadVersion) {
		t.Errorf("DecodeWire(bad version) = %v, want errWireBadVersion", err)
	}
}

// TestWire_RejectsAllDigitRoot per R-041: a wire whose root label is
// all-digit is rejected with errWireBadAllDigit, even if the labels
// are individually well-formed.
func TestWire_RejectsAllDigitRoot(t *testing.T) {
	t.Parallel()
	// schema_version=1 || 0x03 "abc" || 0x03 "123"
	bad := []byte{0x00, 0x01, 0x03, 'a', 'b', 'c', 0x03, '1', '2', '3'}
	_, err := DecodeWire(bad)
	if !errors.Is(err, errWireBadAllDigit) {
		t.Errorf("DecodeWire(all-digit root) = %v, want errWireBadAllDigit", err)
	}
}

// TestWire_RejectsTruncatedLabel per R-041: a declared label length
// that exceeds the remaining input is rejected.
func TestWire_RejectsTruncatedLabel(t *testing.T) {
	t.Parallel()
	bad := []byte{0x00, 0x01, 0x0a, 'a', 'b'} // declares 10-byte label, only 2 bytes
	_, err := DecodeWire(bad)
	if !errors.Is(err, errWireLabelTrunc) {
		t.Errorf("DecodeWire(truncated) = %v, want errWireLabelTrunc", err)
	}
}

// TestWire_RejectsZeroLengthLabel per R-041: a zero-length label
// (length byte 0x00) is rejected.
func TestWire_RejectsZeroLengthLabel(t *testing.T) {
	t.Parallel()
	bad := []byte{0x00, 0x01, 0x00, 0x01, 'a'}
	_, err := DecodeWire(bad)
	if !errors.Is(err, errWireLabelLength) {
		t.Errorf("DecodeWire(zero-length) = %v, want errWireLabelLength", err)
	}
}

// MustParse parses or fails the test, used to build fixtures.
func MustParse(t *testing.T, raw string) Name {
	t.Helper()
	n, err := Parse(raw)
	if err != nil {
		t.Fatalf("MustParse(%q): %v", raw, err)
	}
	return n
}
