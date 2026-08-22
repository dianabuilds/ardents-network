package state

import (
	"bytes"
	"encoding/binary"
	"errors"
)

type materialization struct {
	epochDigest [32]byte
	index       uint32
	record      []byte
	siblings    [][32]byte
}

func decodeMaterializations(encoded [][]byte) ([]materialization, error) {
	if len(encoded) > 64 {
		return nil, errors.New("materialization count exceeds 64")
	}
	materials := make([]materialization, len(encoded))
	for index, raw := range encoded {
		if len(raw) == 0 || len(raw) > 35<<10 {
			return nil, errors.New("materialization framing length is invalid")
		}
		value, err := decodeMaterialization(raw)
		if err != nil {
			return nil, err
		}
		materials[index] = value
	}
	return materials, nil
}

func decodeMaterialization(raw []byte) (materialization, error) {
	d := newDecoder(raw)
	digest, err := d.bytes(32)
	if err != nil {
		return materialization{}, err
	}
	var value materialization
	copy(value.epochDigest[:], digest)
	if value.index, err = d.uint32(); err != nil {
		return materialization{}, err
	}
	record, err := lengthBytes(&d, maximumRecordBytes)
	if err != nil {
		return materialization{}, err
	}
	value.record = append([]byte(nil), record...)
	count, err := d.uint16()
	if err != nil {
		return materialization{}, err
	}
	if count > 64 {
		return materialization{}, errors.New("materialization proof count is invalid")
	}
	for range int(count) {
		sibling, readErr := d.bytes(32)
		if readErr != nil {
			return materialization{}, readErr
		}
		var siblingDigest [32]byte
		copy(siblingDigest[:], sibling)
		value.siblings = append(value.siblings, siblingDigest)
	}
	if !d.done() {
		return materialization{}, errors.New("materialization has trailing bytes")
	}
	return value, nil
}

func encodeMaterialization(value materialization) []byte {
	buffer := new(bytes.Buffer)
	buffer.Write(value.epochDigest[:])
	_ = binary.Write(buffer, binary.BigEndian, value.index)
	writeEpochLengthBytes(buffer, value.record)
	_ = binary.Write(buffer, binary.BigEndian, uint16(len(value.siblings)))
	for _, sibling := range value.siblings {
		buffer.Write(sibling[:])
	}
	return buffer.Bytes()
}

func lengthBytes(d *decoder, maximum int) ([]byte, error) {
	length, err := d.uint32()
	if err != nil {
		return nil, err
	}
	if length == 0 || length > uint32(maximum) {
		return nil, errors.New("materialization member length is invalid")
	}
	return d.bytes(int(length))
}

func writeEpochLengthBytes(buffer *bytes.Buffer, raw []byte) {
	_ = binary.Write(buffer, binary.BigEndian, uint32(len(raw)))
	buffer.Write(raw)
}
