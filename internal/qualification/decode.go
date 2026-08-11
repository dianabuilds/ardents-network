package qualification

import (
	"encoding/binary"
	"errors"
	"unicode/utf8"
)

type byteDecoder struct {
	raw    []byte
	offset int
}

func (d *byteDecoder) take(length int) ([]byte, error) {
	if length < 0 || length > len(d.raw)-d.offset {
		return nil, errors.New("truncated canonical bytes")
	}
	value := d.raw[d.offset : d.offset+length]
	d.offset += length
	return value, nil
}

func (d *byteDecoder) one() (byte, error) {
	value, err := d.take(1)
	if err != nil {
		return 0, err
	}
	return value[0], nil
}

func (d *byteDecoder) u16() (uint16, error) {
	value, err := d.take(2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(value), nil
}

func (d *byteDecoder) u32() (uint32, error) {
	value, err := d.take(4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(value), nil
}

func (d *byteDecoder) u64() (uint64, error) {
	value, err := d.take(8)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(value), nil
}

func (d *byteDecoder) i64() (int64, error) {
	value, err := d.u64()
	return int64(value), err
}

func (d *byteDecoder) text(maximum int) (string, error) {
	length, err := d.one()
	if err != nil {
		return "", err
	}
	if length == 0 || int(length) > maximum {
		return "", errors.New("canonical text length is invalid")
	}
	value, err := d.take(int(length))
	if err != nil {
		return "", err
	}
	if !utf8.Valid(value) {
		return "", errors.New("canonical text is not UTF-8")
	}
	for _, current := range value {
		if current < 0x21 || current > 0x7e {
			return "", errors.New("canonical text contains a non-printing byte")
		}
	}
	return string(value), nil
}
