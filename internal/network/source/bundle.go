package source

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/canonicalcursor"
	epoch2 "github.com/dianabuilds/ardents-network/internal/network/epoch"
)

// Bundle is the private Source response payload. Its members are opaque to
// Source; State verifies their Epoch authority and exact materialization.
// DecodeBundle returns owned copies of every member.
type Bundle struct {
	Epoch     []byte
	Inputs    [][]byte
	Materials [][]byte
}

// EncodeBundle frames one selected materialization without choosing or
// verifying it. The caller supplies bytes from its authenticated decision.
func EncodeBundle(bundle Bundle) ([]byte, error) {
	if len(bundle.Epoch) == 0 || len(bundle.Epoch) > epoch2.MaxEpochBytes || len(bundle.Inputs) > 64 || len(bundle.Materials) != 1 {
		return nil, errors.New("source bundle members are invalid")
	}
	size := 8 + 4 + len(bundle.Epoch) + 2 + 2
	for _, input := range bundle.Inputs {
		if len(input) == 0 || len(input) > epoch2.MaxRecordBytes {
			return nil, errors.New("source bundle input length is invalid")
		}
		size += 4 + len(input)
	}
	for _, material := range bundle.Materials {
		if len(material) == 0 || len(material) > epoch2.MaxMaterializationBytes {
			return nil, errors.New("source bundle materialization length is invalid")
		}
		size += 4 + len(material)
	}
	if size > maximumPayloadBytes {
		return nil, errors.New("source bundle exceeds its state bound")
	}
	buffer := new(bytes.Buffer)
	buffer.Grow(size)
	buffer.WriteString("ARDH3B1\x00")
	writeBundleLengthBytes(buffer, bundle.Epoch)
	writeBundleUint16(buffer, uint16(len(bundle.Inputs)))
	for _, input := range bundle.Inputs {
		writeBundleLengthBytes(buffer, input)
	}
	writeBundleUint16(buffer, uint16(len(bundle.Materials)))
	for _, material := range bundle.Materials {
		writeBundleLengthBytes(buffer, material)
	}
	return buffer.Bytes(), nil
}

// DecodeBundle checks only private wire framing. State checks the decoded
// members against its current Epoch, authorities, and requested material.
func DecodeBundle(raw []byte) (Bundle, error) {
	bundle, err := decodeBundle(raw)
	if err != nil {
		return Bundle{}, fmt.Errorf("%w: %w", ErrFraming, err)
	}
	return bundle, nil
}

func decodeBundle(raw []byte) (Bundle, error) {
	if len(raw) == 0 || len(raw) > maximumPayloadBytes {
		return Bundle{}, errors.New("source bundle length is invalid")
	}
	d := canonicalcursor.NewDecoder(raw)
	magic, err := d.Bytes(8)
	if err != nil || string(magic) != "ARDH3B1\x00" {
		return Bundle{}, errors.New("source bundle magic is invalid")
	}
	epochBytes, err := readBundleLengthBytes(&d, epoch2.MaxEpochBytes)
	if err != nil {
		return Bundle{}, err
	}
	count, err := d.Uint16()
	if err != nil || count > 64 {
		return Bundle{}, errors.New("source bundle input count is invalid")
	}
	bundle := Bundle{Epoch: append([]byte(nil), epochBytes...), Inputs: make([][]byte, 0, count)}
	for range int(count) {
		input, readErr := readBundleLengthBytes(&d, epoch2.MaxRecordBytes)
		if readErr != nil {
			return Bundle{}, readErr
		}
		bundle.Inputs = append(bundle.Inputs, append([]byte(nil), input...))
	}
	materialCount, err := d.Uint16()
	if err != nil || materialCount > 64 {
		return Bundle{}, errors.New("source bundle materialization count is invalid")
	}
	bundle.Materials = make([][]byte, 0, materialCount)
	for range int(materialCount) {
		material, readErr := readBundleLengthBytes(&d, epoch2.MaxMaterializationBytes)
		if readErr != nil {
			return Bundle{}, readErr
		}
		bundle.Materials = append(bundle.Materials, append([]byte(nil), material...))
	}
	if !d.Done() {
		return Bundle{}, errors.New("source bundle has trailing bytes")
	}
	return bundle, nil
}

func readBundleLengthBytes(d *canonicalcursor.Decoder, maximum int) ([]byte, error) {
	length, err := d.Uint32()
	if err != nil || length == 0 || length > uint32(maximum) {
		return nil, errors.New("source bundle member length is invalid")
	}
	return d.Bytes(int(length))
}

func writeBundleLengthBytes(buffer *bytes.Buffer, raw []byte) {
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
