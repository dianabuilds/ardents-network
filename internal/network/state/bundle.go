package state

import (
	"bytes"
	"encoding/binary"
	"errors"
)

const maximumSourceBundleBytes = 1 << 20

type sourceBundle struct {
	epoch     []byte
	inputs    [][]byte
	materials [][]byte
}

func encodeSourceBundle(decision candidateDecision, materialIndex uint32) ([]byte, error) {
	material, err := decision.verified.Materialization(materialIndex)
	if err != nil {
		return nil, err
	}
	buffer := new(bytes.Buffer)
	buffer.WriteString("ARDH3B1\x00")
	writeLengthBytes(buffer, decision.epochBytes)
	writeBundleUint16(buffer, uint16(len(decision.inputs)))
	for _, input := range decision.inputs {
		writeLengthBytes(buffer, input)
	}
	writeBundleUint16(buffer, 1)
	writeLengthBytes(buffer, material)
	if buffer.Len() > maximumSourceBundleBytes {
		return nil, errors.New("source bundle exceeds its state bound")
	}
	return buffer.Bytes(), nil
}

func decodeSourceBundle(raw []byte) (sourceBundle, error) {
	if len(raw) == 0 || len(raw) > maximumSourceBundleBytes {
		return sourceBundle{}, errors.New("source bundle length is invalid")
	}
	d := newDecoder(raw)
	magic, err := d.bytes(8)
	if err != nil || string(magic) != "ARDH3B1\x00" {
		return sourceBundle{}, errors.New("source bundle magic is invalid")
	}
	epoch, err := decodeLengthBytes(&d, maximumEpochBytes)
	if err != nil {
		return sourceBundle{}, err
	}
	count, err := d.uint16()
	if err != nil || count > 64 {
		return sourceBundle{}, errors.New("source bundle input count is invalid")
	}
	bundle := sourceBundle{epoch: append([]byte(nil), epoch...)}
	for range int(count) {
		input, readErr := decodeLengthBytes(&d, maximumRecordBytes)
		if readErr != nil {
			return sourceBundle{}, readErr
		}
		bundle.inputs = append(bundle.inputs, append([]byte(nil), input...))
	}
	materialCount, err := d.uint16()
	if err != nil || materialCount > 64 {
		return sourceBundle{}, errors.New("source bundle materialization count is invalid")
	}
	for range int(materialCount) {
		encoded, readErr := decodeLengthBytes(&d, 35<<10)
		if readErr != nil {
			return sourceBundle{}, readErr
		}
		bundle.materials = append(bundle.materials, append([]byte(nil), encoded...))
	}
	if !d.done() {
		return sourceBundle{}, errors.New("source bundle has trailing bytes")
	}
	return bundle, nil
}

func decodeLengthBytes(d *decoder, maximum int) ([]byte, error) {
	length, err := d.uint32()
	if err != nil || length == 0 || length > uint32(maximum) {
		return nil, errors.New("source bundle member length is invalid")
	}
	return d.bytes(int(length))
}

func writeLengthBytes(buffer *bytes.Buffer, raw []byte) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(raw)))
	buffer.Write(length[:])
	buffer.Write(raw)
}

func writeBundleUint16(buffer *bytes.Buffer, value uint16) {
	var encoded [2]byte
	binary.BigEndian.PutUint16(encoded[:], value)
	buffer.Write(encoded[:])
}

func materializationIndex(raw []byte) (uint32, error) {
	if len(raw) < 36 || len(raw) > 35<<10 {
		return 0, errors.New("materialization framing length is invalid")
	}
	return binary.BigEndian.Uint32(raw[32:36]), nil
}
