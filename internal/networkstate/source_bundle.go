package networkstate

import (
	"bytes"
	"encoding/binary"
	"errors"
)

const maximumSourceBundleBytes = 1 << 20

type sourceBundle struct {
	epoch     []byte
	inputs    [][]byte
	materials []materialization
}

func encodeSourceBundle(decision candidateDecision, materialIndex uint32) ([]byte, error) {
	if materialIndex >= uint32(len(decision.accepted)) {
		return nil, errors.New("requested materialization index is unavailable")
	}
	accepted := make([][]byte, len(decision.accepted))
	for index := range decision.accepted {
		accepted[index] = decision.accepted[index].raw
	}
	material := materialization{
		EpochDigest: decision.epoch.digest, Index: materialIndex,
		Record:   append([]byte(nil), accepted[materialIndex]...),
		Siblings: merkleProofFor(accepted, int(materialIndex)),
	}
	buffer := new(bytes.Buffer)
	buffer.WriteString("ARDH3B1\x00")
	writeLengthBytes(buffer, decision.epochBytes)
	writeBundleUint16(buffer, uint16(len(decision.inputs)))
	for _, input := range decision.inputs {
		writeLengthBytes(buffer, input)
	}
	writeBundleUint16(buffer, 1)
	writeLengthBytes(buffer, encodeMaterialization(material))
	if buffer.Len() > maximumSourceBundleBytes {
		return nil, errors.New("source bundle exceeds its state bound")
	}
	return buffer.Bytes(), nil
}

func decodeSourceBundle(raw []byte) (sourceBundle, error) {
	if len(raw) == 0 || len(raw) > maximumSourceBundleBytes {
		return sourceBundle{}, errors.New("source bundle length is invalid")
	}
	d := decoder{raw: raw}
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
		material, decodeErr := decodeMaterialization(encoded)
		if decodeErr != nil {
			return sourceBundle{}, decodeErr
		}
		bundle.materials = append(bundle.materials, material)
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

func encodeMaterialization(material materialization) []byte {
	buffer := new(bytes.Buffer)
	buffer.Write(material.EpochDigest[:])
	var index [4]byte
	binary.BigEndian.PutUint32(index[:], material.Index)
	buffer.Write(index[:])
	writeLengthBytes(buffer, material.Record)
	writeBundleUint16(buffer, uint16(len(material.Siblings)))
	for _, sibling := range material.Siblings {
		buffer.Write(sibling[:])
	}
	return buffer.Bytes()
}

func decodeMaterialization(raw []byte) (materialization, error) {
	d := decoder{raw: raw}
	value, err := d.bytes(32)
	if err != nil {
		return materialization{}, err
	}
	var material materialization
	copy(material.EpochDigest[:], value)
	if material.Index, err = d.uint32(); err != nil {
		return materialization{}, err
	}
	record, err := decodeLengthBytes(&d, maximumRecordBytes)
	if err != nil {
		return materialization{}, err
	}
	material.Record = append([]byte(nil), record...)
	count, err := d.uint16()
	if err != nil || count > 64 {
		return materialization{}, errors.New("materialization proof count is invalid")
	}
	for range int(count) {
		sibling, readErr := d.bytes(32)
		if readErr != nil {
			return materialization{}, readErr
		}
		var digest [32]byte
		copy(digest[:], sibling)
		material.Siblings = append(material.Siblings, digest)
	}
	if !d.done() {
		return materialization{}, errors.New("materialization has trailing bytes")
	}
	return material, nil
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

func merkleProofFor(values [][]byte, index int) [][32]byte {
	if len(values) <= 1 {
		return nil
	}
	split := merkleSplit(len(values))
	if index < split {
		return append(merkleProofFor(values[:split], index), recordMerkleRoot(values[split:], emptyViewTag))
	}
	return append(merkleProofFor(values[split:], index-split), recordMerkleRoot(values[:split], emptyViewTag))
}
