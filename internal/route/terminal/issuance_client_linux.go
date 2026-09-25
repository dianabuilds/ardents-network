//go:build linux

package terminal

import (
	"encoding/binary"
	"errors"
)

// EncodeIssuanceRequest pads a canonical issuer batch inside the one
// exact 16 KiB terminal OPERATION body.
func EncodeIssuanceRequest(nonce [32]byte, batch []byte) ([]byte, error) {
	if nonce == [32]byte{} || len(batch) == 0 || len(batch) > issuancePayloadSize {
		return nil, errors.New("closed issuance request is invalid")
	}
	body := make([]byte, BodySize)
	body[0] = issuanceOperation
	copy(body[1:33], nonce[:])
	copy(body[33:], batch)
	return body, nil
}

// DecodeIssuanceResult requires the one expected payload length and
// rejects an unbound nonce or alternate response shape.
func DecodeIssuanceResult(body []byte, nonce [32]byte) (IssuanceResult, error) {
	if nonce == [32]byte{} || len(body) != BodySize || body[32] > 4 || binary.BigEndian.Uint32(body[33:37]) != issuanceResultPayload {
		return IssuanceResult{}, errors.New("closed issuance result is invalid")
	}
	result := IssuanceResult{Status: body[32], Payload: append([]byte(nil), body[37:]...)}
	copy(result.Nonce[:], body[:32])
	if result.Nonce != nonce {
		return IssuanceResult{}, errors.New("closed issuance result nonce is invalid")
	}
	return result, nil
}
