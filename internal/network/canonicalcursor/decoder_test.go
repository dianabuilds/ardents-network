package canonicalcursor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestDecoderBytesBounds(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		raw       []byte
		length    int
		want      []byte
		wantError bool
	}{
		{name: "exact fit", raw: []byte{1, 2, 3}, length: 3, want: []byte{1, 2, 3}},
		{name: "prefix", raw: []byte{1, 2, 3}, length: 2, want: []byte{1, 2}},
		{name: "one short", raw: []byte{1, 2, 3}, length: 4, wantError: true},
		{name: "negative", raw: []byte{1, 2, 3}, length: -1, wantError: true},
		{name: "zero at start", raw: []byte{1}, length: 0, want: []byte{}},
		{name: "zero at exact end", raw: []byte{1}, length: 0, want: []byte{}},
		{name: "empty raw", raw: []byte{}, length: 1, wantError: true},
		{name: "nil raw", raw: nil, length: 1, wantError: true},
		{name: "nil raw zero", raw: nil, length: 0, want: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			d := NewDecoder(test.raw)
			if test.name == "zero at exact end" {
				if _, err := d.Bytes(1); err != nil {
					t.Fatal(err)
				}
			}
			got, err := d.Bytes(test.length)
			if (err != nil) != test.wantError {
				t.Fatalf("Bytes(%d) error = %v, wantError=%v", test.length, err, test.wantError)
			}
			if test.wantError {
				if err.Error() != "truncated canonical bytes" {
					t.Fatalf("refusal prose = %q", err.Error())
				}
				if d.Offset() != 0 {
					t.Fatalf("refused read advanced offset to %d", d.Offset())
				}
				return
			}
			if !bytes.Equal(got, test.want) || (got == nil) != (test.want == nil) {
				t.Fatalf("Bytes(%d) = %v (nil=%v), want %v (nil=%v)", test.length, got, got == nil, test.want, test.want == nil)
			}
		})
	}
}

func TestDecoderBigEndianReaders(t *testing.T) {
	t.Parallel()
	raw := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	d := NewDecoder(raw)
	if value, err := d.Byte(); err != nil || value != 0x01 {
		t.Fatalf("Byte() = %x, %v", value, err)
	}
	if value, err := d.Uint16(); err != nil || value != 0x0203 {
		t.Fatalf("Uint16() = %x, %v", value, err)
	}
	if value, err := d.Uint32(); err != nil || value != 0x04050607 {
		t.Fatalf("Uint32() = %x, %v", value, err)
	}
	if value, err := d.Uint64(); err != nil || value != 0x08090a0b0c0d0e0f {
		t.Fatalf("Uint64() = %x, %v", value, err)
	}
	if !d.Done() || d.Offset() != len(raw) {
		t.Fatalf("offset=%d done=%v", d.Offset(), d.Done())
	}
	for _, width := range []int{1, 2, 4, 8} {
		short := NewDecoder(make([]byte, width-1))
		var err error
		switch width {
		case 1:
			_, err = short.Byte()
		case 2:
			_, err = short.Uint16()
		case 4:
			_, err = short.Uint32()
		case 8:
			_, err = short.Uint64()
		}
		if err == nil || err.Error() != "truncated canonical bytes" || short.Offset() != 0 {
			t.Fatalf("width %d refusal = %v, offset %d", width, err, short.Offset())
		}
	}
	negative := NewDecoder([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	if value, err := negative.Int64(); err != nil || value != -1 {
		t.Fatalf("Int64() = %d, %v", value, err)
	}
}

func TestDecoderAliasesSuppliedBytes(t *testing.T) {
	t.Parallel()
	raw := []byte{1, 2, 3, 4}
	d := NewDecoder(raw)
	value, err := d.Bytes(4)
	if err != nil {
		t.Fatal(err)
	}
	raw[0] = 9
	if value[0] != 9 {
		t.Fatal("Bytes must alias the supplied slice, not copy it")
	}
}

// legacyCursor is the pre-R5 private bounded cursor contract, kept verbatim as
// the equivalence oracle. The four canonical grammars (epoch, closedprofile,
// state distribution journal, source bundle) each carried a byte-identical
// copy of this cursor before the shared decoder replaced them.
type legacyCursor struct {
	raw    []byte
	offset int
}

func (d *legacyCursor) bytes(length int) ([]byte, error) {
	if length < 0 || length > len(d.raw)-d.offset {
		return nil, errors.New("truncated canonical bytes")
	}
	value := d.raw[d.offset : d.offset+length]
	d.offset += length
	return value, nil
}

func (d *legacyCursor) byte() (byte, error) {
	value, err := d.bytes(1)
	if err != nil {
		return 0, err
	}
	return value[0], nil
}

func (d *legacyCursor) uint16() (uint16, error) {
	value, err := d.bytes(2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(value), nil
}

func (d *legacyCursor) uint32() (uint32, error) {
	value, err := d.bytes(4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(value), nil
}

func (d *legacyCursor) uint64() (uint64, error) {
	value, err := d.bytes(8)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(value), nil
}

func (d *legacyCursor) int64() (int64, error) {
	value, err := d.uint64()
	return int64(value), err
}

func (d *legacyCursor) done() bool { return d.offset == len(d.raw) }

// TestDecoderMatchesLegacyCursorContract drives identical deterministic
// operation sequences through the shared decoder and the verbatim legacy
// cursor, comparing every value, the exact refusal prose, the consumed offset
// and the done observation after each step, including refused reads.
func TestDecoderMatchesLegacyCursorContract(t *testing.T) {
	t.Parallel()
	state := uint64(0x52352034383034)
	next := func(bound uint64) uint64 {
		state = state*6364136223846793005 + 1442695040888963407
		return (state >> 33) % bound
	}
	for sequence := 0; sequence < 256; sequence++ {
		length := int(next(129))
		raw := make([]byte, length)
		for index := range raw {
			raw[index] = byte(next(256))
		}
		if sequence%16 == 0 {
			raw = nil
		}
		shared := NewDecoder(raw)
		legacy := legacyCursor{raw: raw}
		for step := 0; step < 64; step++ {
			operation := next(6)
			var sharedValue, legacyValue any
			var sharedErr, legacyErr error
			switch operation {
			case 0:
				request := int(next(40)) - 3
				if next(4) == 0 {
					request = len(raw) - legacy.offset + int(next(3)) - 1
				}
				var sharedBytes, legacyBytes []byte
				sharedBytes, sharedErr = shared.Bytes(request)
				legacyBytes, legacyErr = legacy.bytes(request)
				if !bytes.Equal(sharedBytes, legacyBytes) || (sharedBytes == nil) != (legacyBytes == nil) {
					t.Fatalf("sequence %d step %d Bytes(%d): shared %v (nil=%v), legacy %v (nil=%v)", sequence, step, request, sharedBytes, sharedBytes == nil, legacyBytes, legacyBytes == nil)
				}
			case 1:
				var sharedByte, legacyByte byte
				sharedByte, sharedErr = shared.Byte()
				legacyByte, legacyErr = legacy.byte()
				sharedValue, legacyValue = sharedByte, legacyByte
			case 2:
				var shared16, legacy16 uint16
				shared16, sharedErr = shared.Uint16()
				legacy16, legacyErr = legacy.uint16()
				sharedValue, legacyValue = shared16, legacy16
			case 3:
				var shared32, legacy32 uint32
				shared32, sharedErr = shared.Uint32()
				legacy32, legacyErr = legacy.uint32()
				sharedValue, legacyValue = shared32, legacy32
			case 4:
				var shared64, legacy64 uint64
				shared64, sharedErr = shared.Uint64()
				legacy64, legacyErr = legacy.uint64()
				sharedValue, legacyValue = shared64, legacy64
			case 5:
				var sharedInt, legacyInt int64
				sharedInt, sharedErr = shared.Int64()
				legacyInt, legacyErr = legacy.int64()
				sharedValue, legacyValue = sharedInt, legacyInt
			}
			if (sharedErr == nil) != (legacyErr == nil) || sharedErr != nil && sharedErr.Error() != legacyErr.Error() {
				t.Fatalf("sequence %d step %d op %d: shared err %v, legacy err %v", sequence, step, operation, sharedErr, legacyErr)
			}
			if sharedValue != legacyValue {
				t.Fatalf("sequence %d step %d op %d: shared %v, legacy %v", sequence, step, operation, sharedValue, legacyValue)
			}
			if shared.Offset() != legacy.offset || shared.Done() != legacy.done() {
				t.Fatalf("sequence %d step %d: offset %d/%d done %v/%v", sequence, step, shared.Offset(), legacy.offset, shared.Done(), legacy.done())
			}
		}
	}
}
