//go:build linux

package route

import (
	"encoding/binary"
	"errors"
)

// EncodeClosedIssuanceRequest pads a canonical issuer batch inside the one
// exact 16 KiB terminal OPERATION body.
func EncodeClosedIssuanceRequest(nonce [32]byte, batch []byte) ([]byte, error) {
	if nonce == [32]byte{} || len(batch) == 0 || len(batch) > closedIssuancePayloadSize {
		return nil, errors.New("closed issuance request is invalid")
	}
	body := make([]byte, closedTerminalOperationSize)
	body[0] = closedIssuanceOperation
	copy(body[1:33], nonce[:])
	copy(body[33:], batch)
	return body, nil
}

// DecodeClosedIssuanceResult requires the one expected payload length and
// rejects an unbound nonce or alternate response shape.
func DecodeClosedIssuanceResult(body []byte, nonce [32]byte) (ClosedIssuanceResult, error) {
	if nonce == [32]byte{} || len(body) != closedTerminalOperationSize || body[32] > 4 || binary.BigEndian.Uint32(body[33:37]) != closedIssuanceResultPayload {
		return ClosedIssuanceResult{}, errors.New("closed issuance result is invalid")
	}
	result := ClosedIssuanceResult{Status: body[32], Payload: append([]byte(nil), body[37:]...)}
	copy(result.Nonce[:], body[:32])
	if result.Nonce != nonce {
		return ClosedIssuanceResult{}, errors.New("closed issuance result nonce is invalid")
	}
	return result, nil
}
