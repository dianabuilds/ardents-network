package namespace

import (
	"encoding/binary"
	"errors"
)

const leafSchema uint16 = 1

func encodeLeaf(value resolutionLeaf) []byte {
	out := appendUint16(nil, leafSchema)
	out = appendUint32(out, uint32(len(value.signedRecord)))
	out = append(out, value.signedRecord...)
	out = append(out, value.lineageRoot[:]...)
	out = append(out, value.lineageCount, value.state)
	return appendUint64(out, uint64(value.notAfter))
}

func decodeLeaf(raw []byte) (resolutionLeaf, error) {
	cursor := byteCursor{raw: raw}
	schema, schemaErr := cursor.uint16()
	size, sizeErr := cursor.uint32()
	signed, signedErr := cursor.bytes(int(size))
	lineageRoot, rootErr := cursor.array32()
	lineageCount, countErr := cursor.byte()
	state, stateErr := cursor.byte()
	notAfter, timeErr := cursor.uint64()
	if schemaErr != nil || sizeErr != nil || signedErr != nil || rootErr != nil || countErr != nil ||
		stateErr != nil || timeErr != nil || !cursor.done() || schema != leafSchema || size == 0 ||
		lineageCount > 126 || state > 2 || notAfter > uint64(^uint64(0)>>1) {
		return resolutionLeaf{}, errors.New("naming materialization leaf is invalid")
	}
	value := resolutionLeaf{signedRecord: append([]byte(nil), signed...), lineageRoot: lineageRoot,
		lineageCount: lineageCount, state: state, notAfter: int64(notAfter)}
	if string(encodeLeaf(value)) != string(raw) {
		return resolutionLeaf{}, errors.New("naming materialization leaf is non-canonical")
	}
	return value, nil
}

func appendUint16(out []byte, value uint16) []byte { return binary.BigEndian.AppendUint16(out, value) }
func appendUint32(out []byte, value uint32) []byte { return binary.BigEndian.AppendUint32(out, value) }
func appendUint64(out []byte, value uint64) []byte { return binary.BigEndian.AppendUint64(out, value) }
func appendMaterializationText(out []byte, value string) []byte {
	out = appendUint16(out, uint16(len(value)))
	return append(out, value...)
}

type byteCursor struct {
	raw    []byte
	offset int
}

func (cursor *byteCursor) remaining() int { return len(cursor.raw) - cursor.offset }
func (cursor *byteCursor) done() bool     { return cursor.offset == len(cursor.raw) }
func (cursor *byteCursor) bytes(size int) ([]byte, error) {
	if size < 0 || size > cursor.remaining() {
		return nil, errors.New("naming materialization is truncated")
	}
	value := cursor.raw[cursor.offset : cursor.offset+size]
	cursor.offset += size
	return value, nil
}
func (cursor *byteCursor) byte() (byte, error) {
	value, err := cursor.bytes(1)
	if err != nil {
		return 0, err
	}
	return value[0], nil
}
func (cursor *byteCursor) uint16() (uint16, error) {
	value, err := cursor.bytes(2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(value), nil
}
func (cursor *byteCursor) uint32() (uint32, error) {
	value, err := cursor.bytes(4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(value), nil
}
func (cursor *byteCursor) uint64() (uint64, error) {
	value, err := cursor.bytes(8)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(value), nil
}
func (cursor *byteCursor) array32() ([32]byte, error) {
	var value [32]byte
	raw, err := cursor.bytes(32)
	if err != nil {
		return value, err
	}
	copy(value[:], raw)
	return value, nil
}
func (cursor *byteCursor) text(maximum int) (string, error) {
	size, err := cursor.uint16()
	if err != nil || int(size) > maximum {
		return "", errors.New("naming materialization text is invalid")
	}
	value, err := cursor.bytes(int(size))
	return string(value), err
}
