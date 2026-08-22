package state

import (
	"encoding/binary"
	"errors"
	"unicode/utf8"
)

type decoder struct {
	*stateReader
	length int
}

func newDecoder(raw []byte) decoder {
	return decoder{stateReader: &stateReader{raw: raw}, length: len(raw)}
}

func (d *decoder) bytes(length int) ([]byte, error) { return d.Bytes(length) }
func (d *decoder) byte() (byte, error) {
	value, err := d.Bytes(1)
	if err != nil {
		return 0, err
	}
	return value[0], nil
}
func (d *decoder) uint16() (uint16, error) { return d.Uint16() }
func (d *decoder) uint32() (uint32, error) { return d.Uint32() }
func (d *decoder) uint64() (uint64, error) { return d.Uint64() }
func (d *decoder) done() bool              { return d.Consumed() == d.length }

// stateReader owns State's bounded cursor over its authenticated bytes.
type stateReader struct {
	raw    []byte
	offset int
}

func (reader *stateReader) Bytes(length int) ([]byte, error) {
	if length < 0 || length > len(reader.raw)-reader.offset {
		return nil, errors.New("truncated canonical bytes")
	}
	value := reader.raw[reader.offset : reader.offset+length]
	reader.offset += length
	return value, nil
}

func (reader *stateReader) Uint16() (uint16, error) {
	value, err := reader.Bytes(2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(value), err
}

func (reader *stateReader) Uint32() (uint32, error) {
	value, err := reader.Bytes(4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(value), err
}

func (reader *stateReader) Uint64() (uint64, error) {
	value, err := reader.Bytes(8)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(value), err
}

func (reader *stateReader) Consumed() int { return reader.offset }

func (reader *stateReader) Text(maximum int) (string, error) {
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
