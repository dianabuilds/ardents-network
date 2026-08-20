package stage6verify

import (
	"encoding/binary"
	"errors"
)

type namespaceCursor struct {
	raw    []byte
	offset int
}

func (cursor *namespaceCursor) done() bool { return cursor.offset == len(cursor.raw) }
func (cursor *namespaceCursor) take(size int) ([]byte, error) {
	if size < 0 || len(cursor.raw)-cursor.offset < size {
		return nil, errors.New("namespace bytes are truncated")
	}
	value := cursor.raw[cursor.offset : cursor.offset+size]
	cursor.offset += size
	return value, nil
}
func (cursor *namespaceCursor) u8() (byte, error) {
	value, err := cursor.take(1)
	if err != nil {
		return 0, err
	}
	return value[0], nil
}
func (cursor *namespaceCursor) u16() (uint16, error) {
	value, err := cursor.take(2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(value), nil
}
func (cursor *namespaceCursor) u32() (uint32, error) {
	value, err := cursor.take(4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(value), nil
}
func (cursor *namespaceCursor) u64() (uint64, error) {
	value, err := cursor.take(8)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(value), nil
}
func (cursor *namespaceCursor) a32() ([32]byte, error) {
	var out [32]byte
	value, err := cursor.take(32)
	copy(out[:], value)
	return out, err
}
func (cursor *namespaceCursor) text() (string, error) {
	size, err := cursor.u16()
	if err != nil || size > 64 {
		return "", errors.New("namespace text is invalid")
	}
	value, err := cursor.take(int(size))
	return string(value), err
}

func appendText16(out []byte, value string) []byte {
	out = binary.BigEndian.AppendUint16(out, uint16(len(value)))
	return append(out, value...)
}
