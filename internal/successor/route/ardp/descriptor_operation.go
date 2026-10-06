package ardp

import (
	"encoding/binary"
	"errors"
)

const (
	DescriptorLookupBodySize   = 4096
	DescriptorPublishBodySize  = 16384
	DescriptorResultBodySize   = 16384
	MaximumDescriptorProofSize = 15000
)

// DescriptorOperation identifies the single operation of a purpose-3 terminal.
// It grants no authority to read a Target or publish a signed proof.
type DescriptorOperation uint8

const (
	DescriptorLookup  DescriptorOperation = 2
	DescriptorPublish DescriptorOperation = 6
)

// DescriptorRequest carries either an independently selected lookup Target or
// an opaque publication proof. Reachability alone verifies the proof and floors.
type DescriptorRequest struct {
	Operation     DescriptorOperation
	Nonce, Target [32]byte
	Proof         []byte
}

// EncodeDescriptorRequest returns one fixed operation body with zero padding.
// A mixed lookup/publication input refuses rather than discarding a field.
func EncodeDescriptorRequest(request DescriptorRequest) ([]byte, error) {
	if request.Nonce == [32]byte{} {
		return nil, errors.New("route Descriptor request nonce invalid")
	}
	var body []byte
	switch request.Operation {
	case DescriptorLookup:
		if request.Target == [32]byte{} || len(request.Proof) != 0 {
			return nil, errors.New("route Descriptor lookup input invalid")
		}
		body = make([]byte, DescriptorLookupBodySize)
		copy(body[33:65], request.Target[:])
	case DescriptorPublish:
		if request.Target != [32]byte{} || len(request.Proof) == 0 || len(request.Proof) > MaximumDescriptorProofSize {
			return nil, errors.New("route Descriptor publication input invalid")
		}
		body = make([]byte, DescriptorPublishBodySize)
		binary.BigEndian.PutUint16(body[33:35], uint16(len(request.Proof)))
		copy(body[35:], request.Proof)
	default:
		return nil, errors.New("route Descriptor operation invalid")
	}
	body[0] = byte(request.Operation)
	copy(body[1:33], request.Nonce[:])
	return body, nil
}

// DecodeDescriptorRequest borrows a publication proof from the exact body.
// The caller must retain that body until verification and Store use finish.
func DecodeDescriptorRequest(body []byte) (DescriptorRequest, error) {
	if len(body) != DescriptorLookupBodySize && len(body) != DescriptorPublishBodySize {
		return DescriptorRequest{}, errors.New("route Descriptor operation length invalid")
	}
	request := DescriptorRequest{Operation: DescriptorOperation(body[0])}
	copy(request.Nonce[:], body[1:33])
	if request.Nonce == [32]byte{} {
		return DescriptorRequest{}, errors.New("route Descriptor request nonce invalid")
	}
	switch request.Operation {
	case DescriptorLookup:
		if len(body) != DescriptorLookupBodySize || !descriptorPaddingZero(body[65:]) {
			return DescriptorRequest{}, errors.New("route Descriptor lookup length or padding invalid")
		}
		copy(request.Target[:], body[33:65])
		if request.Target == [32]byte{} {
			return DescriptorRequest{}, errors.New("route Descriptor lookup Target invalid")
		}
	case DescriptorPublish:
		length := int(binary.BigEndian.Uint16(body[33:35]))
		if len(body) != DescriptorPublishBodySize || length == 0 || length > MaximumDescriptorProofSize || !descriptorPaddingZero(body[35+length:]) {
			return DescriptorRequest{}, errors.New("route Descriptor publication length or padding invalid")
		}
		request.Proof = body[35 : 35+length]
	default:
		return DescriptorRequest{}, errors.New("route Descriptor operation invalid")
	}
	return request, nil
}

// EncodeDescriptorResult preserves the operation's payload rule: successful
// lookup requires a proof; publication and all refused results are empty.
func EncodeDescriptorResult(operation DescriptorOperation, nonce [32]byte, status uint8, proof []byte) ([]byte, error) {
	if nonce == [32]byte{} || !validDescriptorResult(operation, status, len(proof)) {
		return nil, errors.New("route Descriptor result invalid")
	}
	body := make([]byte, DescriptorResultBodySize)
	copy(body[:32], nonce[:])
	body[32] = status
	binary.BigEndian.PutUint32(body[33:37], uint32(len(proof)))
	copy(body[37:], proof)
	return body, nil
}

// DecodeDescriptorResult checks the original operation and channel-local nonce.
// It borrows proof bytes; no signature, currentness or readiness is established.
func DecodeDescriptorResult(body []byte, operation DescriptorOperation, nonce [32]byte) (uint8, []byte, error) {
	if len(body) != DescriptorResultBodySize || nonce == [32]byte{} {
		return 0, nil, errors.New("route Descriptor result length or nonce invalid")
	}
	var actual [32]byte
	copy(actual[:], body[:32])
	length := uint64(binary.BigEndian.Uint32(body[33:37]))
	if actual != nonce || length > MaximumDescriptorProofSize || !validDescriptorResult(operation, body[32], int(length)) || !descriptorPaddingZero(body[37+length:]) {
		return 0, nil, errors.New("route Descriptor result binding or padding invalid")
	}
	return body[32], body[37 : 37+length], nil
}

func validDescriptorResult(operation DescriptorOperation, status uint8, length int) bool {
	if (operation != DescriptorLookup && operation != DescriptorPublish) || status > 4 || length < 0 || length > MaximumDescriptorProofSize {
		return false
	}
	if status == 0 && operation == DescriptorLookup {
		return length != 0
	}
	return length == 0
}

func descriptorPaddingZero(body []byte) bool {
	for _, value := range body {
		if value != 0 {
			return false
		}
	}
	return true
}
