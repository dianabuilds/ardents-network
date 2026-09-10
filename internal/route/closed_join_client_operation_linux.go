//go:build linux

package route

import (
	"encoding/binary"
	"errors"
)

func EncodeClosedJoinRequest(request ClosedJoinRequest) ([]byte, error) {
	if !validClosedJoinRequest(request) {
		return nil, errors.New("closed JOIN request invalid")
	}
	body := make([]byte, closedSmallTerminalOperation)
	body[0] = 5
	copy(body[1:33], request.Nonce[:])
	copy(body[33:65], request.Secret[:])
	body[65] = request.Side
	copy(body[66:98], request.Context[:])
	binary.BigEndian.PutUint64(body[98:106], uint64(request.Deadline.Unix()))
	return body, nil
}

func DecodeClosedJoinResult(body []byte, nonce [32]byte) (uint8, error) {
	if len(body) != closedTerminalOperationSize || nonce == [32]byte{} || body[32] > 4 || !closedDescriptorPadding(body[33:]) {
		return 0, errors.New("closed JOIN result or padding invalid")
	}
	var received [32]byte
	copy(received[:], body[:32])
	if received != nonce {
		return 0, errors.New("closed JOIN result nonce differs")
	}
	return body[32], nil
}
