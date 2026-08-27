package private

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/naming"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

const messageVersion uint16 = 1

type request struct {
	nonce    [32]byte
	deadline int64
	link     alpha.ServiceLink
}

type response struct {
	nonce    [32]byte
	deadline int64
	link     alpha.ServiceLink
	corpus   []byte
}

func encodeRequest(value request) ([]byte, error) {
	if value.nonce == [32]byte{} || value.deadline <= 0 {
		return nil, errors.New("alpha private request is invalid")
	}
	nameWire, err := naming.EncodeWire(value.link.Name())
	if err != nil || len(nameWire) > 0xffff {
		return nil, errors.New("alpha private request name is invalid")
	}
	out := make([]byte, 0, 2+32+8+2+len(nameWire))
	out = binary.BigEndian.AppendUint16(out, messageVersion)
	out = append(out, value.nonce[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(value.deadline))
	out = binary.BigEndian.AppendUint16(out, uint16(len(nameWire)))
	return append(out, nameWire...), nil
}

func decodeRequest(raw []byte) (request, error) {
	if len(raw) < 44 || binary.BigEndian.Uint16(raw[:2]) != messageVersion {
		return request{}, errors.New("alpha private request schema is invalid")
	}
	var value request
	copy(value.nonce[:], raw[2:34])
	value.deadline = int64(binary.BigEndian.Uint64(raw[34:42]))
	nameLength := int(binary.BigEndian.Uint16(raw[42:44]))
	if value.nonce == [32]byte{} || value.deadline <= 0 || nameLength == 0 || len(raw) != 44+nameLength {
		return request{}, errors.New("alpha private request is malformed")
	}
	name, err := naming.DecodeWire(raw[44:])
	if err != nil {
		return request{}, err
	}
	canonical, err := naming.EncodeWire(name)
	if err != nil || !bytes.Equal(canonical, raw[44:]) {
		return request{}, errors.New("alpha private request name is non-canonical")
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://" + string(name))
	if err != nil {
		return request{}, err
	}
	value.link = link
	return value, nil
}

func encodeResponse(value response) ([]byte, error) {
	if value.nonce == [32]byte{} || value.deadline <= 0 || len(value.corpus) == 0 || len(value.corpus) > 0xffff {
		return nil, errors.New("alpha private response is invalid")
	}
	nameWire, err := naming.EncodeWire(value.link.Name())
	if err != nil || len(nameWire) > 0xffff {
		return nil, errors.New("alpha private response name is invalid")
	}
	out := make([]byte, 0, 2+32+8+2+len(nameWire)+2+len(value.corpus))
	out = binary.BigEndian.AppendUint16(out, messageVersion)
	out = append(out, value.nonce[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(value.deadline))
	out = binary.BigEndian.AppendUint16(out, uint16(len(nameWire)))
	out = append(out, nameWire...)
	out = binary.BigEndian.AppendUint16(out, uint16(len(value.corpus)))
	return append(out, value.corpus...), nil
}

func decodeResponse(raw []byte) (response, error) {
	if len(raw) < 46 || binary.BigEndian.Uint16(raw[:2]) != messageVersion {
		return response{}, errors.New("alpha private response schema is invalid")
	}
	var value response
	copy(value.nonce[:], raw[2:34])
	value.deadline = int64(binary.BigEndian.Uint64(raw[34:42]))
	nameLength := int(binary.BigEndian.Uint16(raw[42:44]))
	if value.nonce == [32]byte{} || value.deadline <= 0 || nameLength == 0 || len(raw) < 46+nameLength {
		return response{}, errors.New("alpha private response is malformed")
	}
	name, err := naming.DecodeWire(raw[44 : 44+nameLength])
	if err != nil {
		return response{}, err
	}
	canonical, err := naming.EncodeWire(name)
	if err != nil || !bytes.Equal(canonical, raw[44:44+nameLength]) {
		return response{}, errors.New("alpha private response name is non-canonical")
	}
	corpusLength := int(binary.BigEndian.Uint16(raw[44+nameLength : 46+nameLength]))
	if corpusLength == 0 || len(raw) != 46+nameLength+corpusLength {
		return response{}, errors.New("alpha private response corpus is malformed")
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://" + string(name))
	if err != nil {
		return response{}, err
	}
	value.link = link
	value.corpus = append([]byte(nil), raw[46+nameLength:]...)
	return value, nil
}

func pad(raw []byte) ([]byte, error) {
	if len(raw) == 0 || len(raw) > fixedMessageSize-2 {
		return nil, errors.New("alpha private message exceeds fixed envelope")
	}
	result := make([]byte, fixedMessageSize)
	binary.BigEndian.PutUint16(result[:2], uint16(len(raw)))
	copy(result[2:], raw)
	return result, nil
}

func unpad(raw []byte) ([]byte, error) {
	if len(raw) != fixedMessageSize {
		return nil, errors.New("alpha private message has wrong fixed envelope")
	}
	length := int(binary.BigEndian.Uint16(raw[:2]))
	if length == 0 || length > fixedMessageSize-2 {
		return nil, errors.New("alpha private message length is invalid")
	}
	for _, value := range raw[2+length:] {
		if value != 0 {
			return nil, errors.New("alpha private message padding is non-canonical")
		}
	}
	return append([]byte(nil), raw[2:2+length]...), nil
}
