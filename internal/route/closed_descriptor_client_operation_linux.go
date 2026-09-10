//go:build linux

package route

import (
	"encoding/binary"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// EncodeClosedDescriptorLookup creates the exact 4096-byte operation body.
func EncodeClosedDescriptorLookup(nonce, target [32]byte) ([]byte, error) {
	if nonce == [32]byte{} || target == [32]byte{} {
		return nil, errors.New("closed Descriptor lookup is invalid")
	}
	body := make([]byte, closedSmallTerminalOperation)
	body[0] = 2
	copy(body[1:33], nonce[:])
	copy(body[33:65], target[:])
	return body, nil
}

// EncodeClosedDescriptorPublication keeps the complete signed proof inside
// the exact 16384-byte operation body; it never substitutes a URL or prefix.
func EncodeClosedDescriptorPublication(nonce [32]byte, descriptor []byte) ([]byte, error) {
	if nonce == [32]byte{} || len(descriptor) == 0 || len(descriptor) > reachability.MaximumPrivateDescriptorSize {
		return nil, errors.New("closed Descriptor publication is invalid")
	}
	body := make([]byte, closedTerminalOperationSize)
	body[0] = 6
	copy(body[1:33], nonce[:])
	binary.BigEndian.PutUint16(body[33:35], uint16(len(descriptor)))
	copy(body[35:], descriptor)
	return body, nil
}
