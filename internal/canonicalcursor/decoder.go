package canonicalcursor

import (
	"encoding/binary"
	"errors"
)

// Decoder bounds reads from one caller-supplied canonical byte slice. It is a
// pure value cursor: reads alias the supplied bytes, never copy, and refuse
// exactly when a read would cross the remaining bound.
type Decoder struct {
	raw    []byte
	offset int
}

// NewDecoder returns a cursor positioned at the start of raw.
func NewDecoder(raw []byte) Decoder { return Decoder{raw: raw} }

// Bytes returns the next length bytes, aliasing the supplied slice. A
// zero-length read is legal at any position, including the exact end.
func (d *Decoder) Bytes(length int) ([]byte, error) {
	if length < 0 || length > len(d.raw)-d.offset {
		return nil, errors.New("truncated canonical bytes")
	}
	value := d.raw[d.offset : d.offset+length]
	d.offset += length
	return value, nil
}

// Byte returns the next single byte.
func (d *Decoder) Byte() (byte, error) {
	value, err := d.Bytes(1)
	if err != nil {
		return 0, err
	}
	return value[0], nil
}

// Uint16 returns the next big-endian uint16.
func (d *Decoder) Uint16() (uint16, error) {
	value, err := d.Bytes(2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(value), nil
}

// Uint32 returns the next big-endian uint32.
func (d *Decoder) Uint32() (uint32, error) {
	value, err := d.Bytes(4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(value), nil
}

// Uint64 returns the next big-endian uint64.
func (d *Decoder) Uint64() (uint64, error) {
	value, err := d.Bytes(8)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(value), nil
}

// Int64 returns the next big-endian uint64 reinterpreted as int64.
func (d *Decoder) Int64() (int64, error) {
	value, err := d.Uint64()
	return int64(value), err
}

// Done reports whether the cursor consumed every supplied byte.
func (d *Decoder) Done() bool { return d.offset == len(d.raw) }

// Offset returns the number of bytes consumed so far.
func (d *Decoder) Offset() int { return d.offset }
