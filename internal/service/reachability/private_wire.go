package reachability

import (
	"encoding/binary"
	"errors"
)

const (
	privateMessageSize = 4096
	privateWireVersion = uint16(1)
	privateResolve     = byte(1)
	privateResolved    = byte(1)
	privateUnavailable = byte(2)
	privateConflicting = byte(3)
	privateInvalid     = byte(4)
)

type privateRequest struct {
	network, target, nonce [32]byte
	deadline               int64
}

type privateResponse struct {
	network, target, nonce [32]byte
	deadline               int64
	class                  byte
	descriptor             []byte
}

func encodePrivateRequest(value privateRequest) ([]byte, error) {
	if value.network == [32]byte{} || value.target == [32]byte{} || value.nonce == [32]byte{} || value.deadline <= 0 {
		return nil, errors.New("private reachability request is invalid")
	}
	out := make([]byte, 0, 2+1+32+32+32+8)
	out = binary.BigEndian.AppendUint16(out, privateWireVersion)
	out = append(out, privateResolve)
	for _, field := range [][32]byte{value.network, value.target, value.nonce} {
		out = append(out, field[:]...)
	}
	return binary.BigEndian.AppendUint64(out, uint64(value.deadline)), nil
}

func decodePrivateRequest(raw []byte) (privateRequest, error) {
	if len(raw) != 2+1+32+32+32+8 || binary.BigEndian.Uint16(raw[:2]) != privateWireVersion || raw[2] != privateResolve {
		return privateRequest{}, errors.New("private reachability request is malformed")
	}
	value := privateRequest{deadline: int64(binary.BigEndian.Uint64(raw[99:]))}
	copy(value.network[:], raw[3:35])
	copy(value.target[:], raw[35:67])
	copy(value.nonce[:], raw[67:99])
	if value.network == [32]byte{} || value.target == [32]byte{} || value.nonce == [32]byte{} || value.deadline <= 0 {
		return privateRequest{}, errors.New("private reachability request content is invalid")
	}
	return value, nil
}

func encodePrivateResponse(value privateResponse) ([]byte, error) {
	if value.network == [32]byte{} || value.target == [32]byte{} || value.nonce == [32]byte{} || value.deadline <= 0 ||
		(value.class != privateResolved && value.class != privateUnavailable && value.class != privateConflicting && value.class != privateInvalid) ||
		len(value.descriptor) > MaximumDescriptorSize || (value.class == privateResolved) != (len(value.descriptor) > 0) {
		return nil, errors.New("private reachability response is invalid")
	}
	out := make([]byte, 0, 2+1+32+32+32+8+1+2+len(value.descriptor))
	out = binary.BigEndian.AppendUint16(out, privateWireVersion)
	out = append(out, privateResolve)
	for _, field := range [][32]byte{value.network, value.target, value.nonce} {
		out = append(out, field[:]...)
	}
	out = binary.BigEndian.AppendUint64(out, uint64(value.deadline))
	out = append(out, value.class)
	out = binary.BigEndian.AppendUint16(out, uint16(len(value.descriptor)))
	return append(out, value.descriptor...), nil
}

func decodePrivateResponse(raw []byte) (privateResponse, error) {
	const header = 2 + 1 + 32 + 32 + 32 + 8 + 1 + 2
	if len(raw) < header || binary.BigEndian.Uint16(raw[:2]) != privateWireVersion || raw[2] != privateResolve {
		return privateResponse{}, errors.New("private reachability response is malformed")
	}
	value := privateResponse{deadline: int64(binary.BigEndian.Uint64(raw[99:])), class: raw[107]}
	copy(value.network[:], raw[3:35])
	copy(value.target[:], raw[35:67])
	copy(value.nonce[:], raw[67:99])
	length := int(binary.BigEndian.Uint16(raw[108:]))
	if value.network == [32]byte{} || value.target == [32]byte{} || value.nonce == [32]byte{} || value.deadline <= 0 ||
		(value.class != privateResolved && value.class != privateUnavailable && value.class != privateConflicting && value.class != privateInvalid) ||
		length > MaximumDescriptorSize || header+length != len(raw) || (value.class == privateResolved) != (length > 0) {
		return privateResponse{}, errors.New("private reachability response content is invalid")
	}
	value.descriptor = append([]byte(nil), raw[header:]...)
	return value, nil
}

func padPrivateMessage(raw []byte) ([]byte, error) {
	if len(raw) == 0 || len(raw) > privateMessageSize-2 {
		return nil, errors.New("private reachability message exceeds fixed envelope")
	}
	out := make([]byte, privateMessageSize)
	binary.BigEndian.PutUint16(out[:2], uint16(len(raw)))
	copy(out[2:], raw)
	return out, nil
}

func unpadPrivateMessage(raw []byte) ([]byte, error) {
	if len(raw) != privateMessageSize {
		return nil, errors.New("private reachability message has wrong size")
	}
	length := int(binary.BigEndian.Uint16(raw[:2]))
	if length == 0 || length > privateMessageSize-2 {
		return nil, errors.New("private reachability message length is invalid")
	}
	for _, value := range raw[2+length:] {
		if value != 0 {
			return nil, errors.New("private reachability message padding is non-canonical")
		}
	}
	return append([]byte(nil), raw[2:2+length]...), nil
}
