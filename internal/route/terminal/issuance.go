package terminal

import (
	"encoding/binary"
	"errors"
)

const (
	issuanceOperation     = uint8(1)
	SmallBodySize         = 4 << 10
	BodySize              = 16 << 10
	issuancePayloadSize   = BodySize - 1 - 32
	issuanceResultHeader  = 32 + 1 + 4
	issuanceResultPayload = BodySize - issuanceResultHeader
)

// IssuanceRequest is the target-free terminal operation that carries
// exactly one padded canonical blind-token batch.
type IssuanceRequest struct {
	Nonce   [32]byte
	Payload []byte
}

// IssuanceResult is one fixed terminal result body. Status follows the
// v3 RESULT vocabulary: accepted=0, unavailable=1, exhausted=2,
// stale/incompatible=3, withdrawn=4.
type IssuanceResult struct {
	Nonce   [32]byte
	Status  uint8
	Payload []byte
}

// DecodeIssuanceRequest parses the fixed operation body without
// interpreting its credential-owned padded batch payload.
func DecodeIssuanceRequest(body []byte) (IssuanceRequest, error) {
	if len(body) != BodySize || body[0] != issuanceOperation {
		return IssuanceRequest{}, errors.New("closed issuance operation is invalid")
	}
	request := IssuanceRequest{Payload: append([]byte(nil), body[33:]...)}
	copy(request.Nonce[:], body[1:33])
	if request.Nonce == [32]byte{} {
		return IssuanceRequest{}, errors.New("closed issuance nonce is invalid")
	}
	return request, nil
}

// EncodeIssuanceResult binds one fixed credential payload to the
// matching request nonce inside the exact 16 KiB terminal RESULT body.
func EncodeIssuanceResult(nonce [32]byte, status uint8, payload []byte) ([]byte, error) {
	if nonce == [32]byte{} || status > 4 || len(payload) != issuanceResultPayload {
		return nil, errors.New("closed issuance result is invalid")
	}
	body := make([]byte, BodySize)
	copy(body[:32], nonce[:])
	body[32] = status
	binary.BigEndian.PutUint32(body[33:37], uint32(len(payload)))
	copy(body[37:], payload)
	return body, nil
}
