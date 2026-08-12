package state

import (
	"encoding/binary"
	"errors"
	"unicode/utf8"
)

type byteDecoder struct {
	raw    []byte
	offset int
}

func (decoder *byteDecoder) take(length int) ([]byte, error) {
	if length < 0 || length > len(decoder.raw)-decoder.offset {
		return nil, errors.New("truncated canonical bytes")
	}
	value := decoder.raw[decoder.offset : decoder.offset+length]
	decoder.offset += length
	return value, nil
}

func (decoder *byteDecoder) one() (byte, error) {
	value, err := decoder.take(1)
	if err != nil {
		return 0, err
	}
	return value[0], nil
}

func (decoder *byteDecoder) u16() (uint16, error) {
	value, err := decoder.take(2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(value), nil
}

func (decoder *byteDecoder) u32() (uint32, error) {
	value, err := decoder.take(4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(value), nil
}

func (decoder *byteDecoder) u64() (uint64, error) {
	value, err := decoder.take(8)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(value), nil
}

func (decoder *byteDecoder) i64() (int64, error) {
	value, err := decoder.u64()
	return int64(value), err
}

func (decoder *byteDecoder) text(maximum int) (string, error) {
	length, err := decoder.one()
	if err != nil {
		return "", err
	}
	if length == 0 || int(length) > maximum {
		return "", errors.New("canonical text length is invalid")
	}
	value, err := decoder.take(int(length))
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
