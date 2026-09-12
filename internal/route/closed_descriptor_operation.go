package route

import (
	"encoding/binary"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// ClosedDescriptorRequest contains exactly one admitted private lookup or
// publication. An empty Descriptor denotes lookup of the nonzero Target.
type ClosedDescriptorRequest struct {
	Nonce, Target [32]byte
	Descriptor    []byte
}

// DecodeClosedDescriptorRequest rejects unsupported operations, combined
// payloads and nonzero padding before the recipient reaches its Store.
func DecodeClosedDescriptorRequest(body []byte) (ClosedDescriptorRequest, error) {
	if len(body) != closedSmallTerminalOperation && len(body) != closedTerminalOperationSize {
		return ClosedDescriptorRequest{}, errors.New("closed Descriptor operation length is invalid")
	}
	var request ClosedDescriptorRequest
	copy(request.Nonce[:], body[1:33])
	if request.Nonce == [32]byte{} {
		return request, errors.New("closed Descriptor operation nonce is invalid")
	}
	switch body[0] {
	case 2:
		if len(body) != closedSmallTerminalOperation || !closedDescriptorPadding(body[65:]) {
			return request, errors.New("closed Descriptor lookup padding is invalid")
		}
		copy(request.Target[:], body[33:65])
		if request.Target == [32]byte{} {
			return request, errors.New("closed Descriptor Target is invalid")
		}
	case 6:
		length := int(binary.BigEndian.Uint16(body[33:35]))
		if len(body) != closedTerminalOperationSize || length == 0 || length > reachability.MaximumPrivateDescriptorSize || !closedDescriptorPadding(body[35+length:]) {
			return request, errors.New("closed Descriptor publication length or padding is invalid")
		}
		request.Descriptor = append([]byte(nil), body[35:35+length]...)
	default:
		return request, errors.New("closed Descriptor operation is unsupported")
	}
	return request, nil
}

// EncodeClosedDescriptorResult pads all outcomes to the same 16384 bytes.
// Only an accepted lookup may carry a proof; publication success is empty.
func EncodeClosedDescriptorResult(nonce [32]byte, status uint8, descriptor []byte) ([]byte, error) {
	if nonce == [32]byte{} || status > 4 || len(descriptor) > reachability.MaximumPrivateDescriptorSize || status != 0 && len(descriptor) != 0 {
		return nil, errors.New("closed Descriptor result is invalid")
	}
	body := make([]byte, closedTerminalOperationSize)
	copy(body[:32], nonce[:])
	body[32] = status
	binary.BigEndian.PutUint32(body[33:37], uint32(len(descriptor)))
	copy(body[37:], descriptor)
	return body, nil
}

// DecodeClosedDescriptorResult binds a complete proof or refusal to this
// channel's exact request nonce. The caller still verifies the signed proof.
func DecodeClosedDescriptorResult(body []byte, nonce [32]byte) (uint8, []byte, error) {
	if len(body) != closedTerminalOperationSize || nonce == [32]byte{} || body[32] > 4 {
		return 0, nil, errors.New("closed Descriptor result length is invalid")
	}
	var received [32]byte
	copy(received[:], body[:32])
	length := uint64(binary.BigEndian.Uint32(body[33:37]))
	if received != nonce || length > reachability.MaximumPrivateDescriptorSize || body[32] != 0 && length != 0 || !closedDescriptorPadding(body[37+length:]) {
		return 0, nil, errors.New("closed Descriptor result binding or padding is invalid")
	}
	return body[32], append([]byte(nil), body[37:37+length]...), nil
}

func closedDescriptorPadding(body []byte) bool {
	for _, value := range body {
		if value != 0 {
			return false
		}
	}
	return true
}
