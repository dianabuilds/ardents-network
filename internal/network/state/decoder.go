package state

import (
	"encoding/binary"
	"errors"
)

// decoder owns State's bounded cursor over canonical control bytes.
type decoder struct {
	raw    []byte
	offset int
}

func newDecoder(raw []byte) decoder { return decoder{raw: raw} }

func (d *decoder) bytes(length int) ([]byte, error) {
	if length < 0 || length > len(d.raw)-d.offset {
		return nil, errors.New("truncated canonical bytes")
	}
	value := d.raw[d.offset : d.offset+length]
	d.offset += length
	return value, nil
}

func (d *decoder) byte() (byte, error) {
	value, err := d.bytes(1)
	if err != nil {
		return 0, err
	}
	return value[0], nil
}

func (d *decoder) uint64() (uint64, error) {
	value, err := d.bytes(8)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(value), nil
}

func (d *decoder) done() bool { return d.offset == len(d.raw) }
