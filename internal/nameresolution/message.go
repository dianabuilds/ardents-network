package nameresolution

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/naming"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

const (
	messageSchema     uint16 = 3
	resolveOperation  byte   = 1
	resultResolved    byte   = 1
	resultUnavailable byte   = 2
)

type resolutionRequest struct {
	network   [32]byte
	nonce     [32]byte
	deadline  int64
	name      string
	admission namespace.Proof
}

type resolutionResponse struct {
	network    [32]byte
	nonce      [32]byte
	deadline   int64
	name       string
	generation uint64
	revision   uint64
	result     byte
	proof      []byte
}

func encodeRequest(value resolutionRequest) ([]byte, error) {
	name, err := naming.Parse(value.name)
	if err != nil || value.network == [32]byte{} || value.nonce == [32]byte{} || value.deadline <= 0 {
		return nil, errors.New("resolution request is invalid")
	}
	wire, err := naming.EncodeWire(name)
	if err != nil || len(wire) > 0xffff {
		return nil, errors.New("resolution name encoding is invalid")
	}
	out := make([]byte, 0, 75+len(wire))
	out = binary.BigEndian.AppendUint16(out, messageSchema)
	out = append(out, resolveOperation)
	out = append(out, value.network[:]...)
	out = append(out, value.nonce[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(value.deadline))
	out = binary.BigEndian.AppendUint16(out, uint16(len(wire)))
	out = append(out, wire...)
	admission, err := encodeAdmissionProof(value.admission)
	if err != nil {
		return nil, err
	}
	out = binary.BigEndian.AppendUint16(out, uint16(len(admission)))
	return append(out, admission...), nil
}

func decodeRequest(raw []byte) (resolutionRequest, error) {
	if len(raw) < 77 || binary.BigEndian.Uint16(raw[:2]) != messageSchema || raw[2] != resolveOperation {
		return resolutionRequest{}, errors.New("resolution request schema is invalid")
	}
	var value resolutionRequest
	copy(value.network[:], raw[3:35])
	copy(value.nonce[:], raw[35:67])
	value.deadline = int64(binary.BigEndian.Uint64(raw[67:75]))
	size := int(binary.BigEndian.Uint16(raw[75:77]))
	if size == 0 || len(raw) < 79+size {
		return resolutionRequest{}, errors.New("resolution request length is invalid")
	}
	name, err := naming.DecodeWire(raw[77 : 77+size])
	if err != nil {
		return resolutionRequest{}, err
	}
	canonical, err := naming.EncodeWire(name)
	if err != nil || !bytes.Equal(canonical, raw[77:77+size]) {
		return resolutionRequest{}, errors.New("resolution request name is non-canonical")
	}
	value.name = string(name)
	proofSize := int(binary.BigEndian.Uint16(raw[77+size : 79+size]))
	if proofSize == 0 || len(raw) != 79+size+proofSize {
		return resolutionRequest{}, errors.New("resolution admission proof length is invalid")
	}
	value.admission, err = decodeAdmissionProof(raw[79+size:])
	if err != nil {
		return resolutionRequest{}, err
	}
	return value, nil
}

func encodeResponse(value resolutionResponse) ([]byte, error) {
	name, nameErr := naming.Parse(value.name)
	nameWire, wireErr := naming.EncodeWire(name)
	if value.network == [32]byte{} || value.nonce == [32]byte{} || value.deadline <= 0 ||
		(value.result != resultResolved && value.result != resultUnavailable) || len(value.proof) > 0xffff ||
		(value.result == resultResolved) != (len(value.proof) > 0) || nameErr != nil || wireErr != nil || len(nameWire) > 0xffff ||
		(value.result == resultResolved) != (value.generation > 0 && value.revision > 0) {
		return nil, errors.New("resolution response is invalid")
	}
	out := make([]byte, 0, 94+len(nameWire))
	out = binary.BigEndian.AppendUint16(out, messageSchema)
	out = append(out, resolveOperation)
	out = append(out, value.network[:]...)
	out = append(out, value.nonce[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(value.deadline))
	out = binary.BigEndian.AppendUint16(out, uint16(len(nameWire)))
	out = append(out, nameWire...)
	out = binary.BigEndian.AppendUint64(out, value.generation)
	out = binary.BigEndian.AppendUint64(out, value.revision)
	out = append(out, value.result)
	out = binary.BigEndian.AppendUint16(out, uint16(len(value.proof)))
	return append(out, value.proof...), nil
}

func decodeResponse(raw []byte) (resolutionResponse, error) {
	if len(raw) < 97 || binary.BigEndian.Uint16(raw[:2]) != messageSchema || raw[2] != resolveOperation {
		return resolutionResponse{}, errors.New("resolution response schema is invalid")
	}
	var value resolutionResponse
	copy(value.network[:], raw[3:35])
	copy(value.nonce[:], raw[35:67])
	value.deadline = int64(binary.BigEndian.Uint64(raw[67:75]))
	nameSize := int(binary.BigEndian.Uint16(raw[75:77]))
	if nameSize == 0 || len(raw) < 96+nameSize {
		return resolutionResponse{}, errors.New("resolution response name is malformed")
	}
	name, err := naming.DecodeWire(raw[77 : 77+nameSize])
	if err != nil {
		return resolutionResponse{}, err
	}
	value.name = string(name)
	offset := 77 + nameSize
	value.generation = binary.BigEndian.Uint64(raw[offset:])
	value.revision = binary.BigEndian.Uint64(raw[offset+8:])
	value.result = raw[offset+16]
	proofSize := int(binary.BigEndian.Uint16(raw[offset+17:]))
	offset += 19
	if proofSize > len(raw)-offset {
		return resolutionResponse{}, errors.New("resolution response proof is malformed")
	}
	value.proof = append([]byte(nil), raw[offset:offset+proofSize]...)
	offset += proofSize
	if offset != len(raw) || (value.result == resultResolved) != (len(value.proof) > 0) ||
		(value.result == resultResolved) != (value.generation > 0 && value.revision > 0) ||
		(value.result != resultResolved && value.result != resultUnavailable) {
		return resolutionResponse{}, errors.New("resolution response is non-canonical")
	}
	return value, nil
}

func padMessage(raw []byte) ([]byte, error) {
	if len(raw) == 0 || len(raw) > fixedMessageSize-2 {
		return nil, errors.New("private resolution message exceeds fixed envelope")
	}
	out := make([]byte, fixedMessageSize)
	binary.BigEndian.PutUint16(out[:2], uint16(len(raw)))
	copy(out[2:], raw)
	return out, nil
}

func unpadMessage(raw []byte) ([]byte, error) {
	if len(raw) != fixedMessageSize {
		return nil, errors.New("private resolution plaintext has wrong size")
	}
	size := int(binary.BigEndian.Uint16(raw[:2]))
	if size == 0 || size > fixedMessageSize-2 {
		return nil, errors.New("private resolution plaintext length is invalid")
	}
	for _, value := range raw[2+size:] {
		if value != 0 {
			return nil, errors.New("private resolution padding is non-canonical")
		}
	}
	return append([]byte(nil), raw[2:2+size]...), nil
}
