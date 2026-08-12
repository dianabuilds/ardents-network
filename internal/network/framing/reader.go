package framing

import (
	"encoding/binary"
	"errors"
	"unicode/utf8"
)

// Reader owns a bounded cursor over immutable canonical bytes.
type Reader struct {
	raw    []byte
	offset int
}

// New returns a reader over raw.
func New(raw []byte) *Reader { return &Reader{raw: raw} }

// Bytes returns the next length bytes.
func (reader *Reader) Bytes(length int) ([]byte, error) {
	if length < 0 || length > len(reader.raw)-reader.offset {
		return nil, errors.New("truncated canonical bytes")
	}
	value := reader.raw[reader.offset : reader.offset+length]
	reader.offset += length
	return value, nil
}

// Uint16 returns the next big-endian value.
func (reader *Reader) Uint16() (uint16, error) {
	value, err := reader.Bytes(2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(value), nil
}

// Uint32 returns the next big-endian value.
func (reader *Reader) Uint32() (uint32, error) {
	value, err := reader.Bytes(4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(value), nil
}

// Uint64 returns the next big-endian value.
func (reader *Reader) Uint64() (uint64, error) {
	value, err := reader.Bytes(8)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(value), nil
}

// Text returns one length-prefixed printable UTF-8 value.
func (reader *Reader) Text(maximum int) (string, error) {
	rawLength, err := reader.Bytes(1)
	if err != nil {
		return "", err
	}
	length := rawLength[0]
	if length == 0 || int(length) > maximum {
		return "", errors.New("canonical text length is invalid")
	}
	value, err := reader.Bytes(int(length))
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

// Consumed returns the number of bytes read.
func (reader *Reader) Consumed() int { return reader.offset }
