package route

import (
	"encoding/binary"
	"errors"
)

const (
	closedIssuanceOperation      = uint8(1)
	closedSmallTerminalOperation = 4 << 10
	closedTerminalOperationSize  = 16 << 10
	closedIssuancePayloadSize    = closedTerminalOperationSize - 1 - 32
	closedIssuanceResultHeader   = 32 + 1 + 4
	closedIssuanceResultPayload  = closedTerminalOperationSize - closedIssuanceResultHeader
)

// ClosedIssuanceRequest is the target-free terminal operation that carries
// exactly one padded canonical blind-token batch.
type ClosedIssuanceRequest struct {
	Nonce   [32]byte
	Payload []byte
}

// ClosedIssuanceResult is one fixed terminal result body. Status follows the
// v3 RESULT vocabulary: accepted=0, unavailable=1, exhausted=2,
// stale/incompatible=3, withdrawn=4.
type ClosedIssuanceResult struct {
	Nonce   [32]byte
	Status  uint8
	Payload []byte
}

// DecodeClosedIssuanceRequest parses the fixed operation body without
// interpreting its credential-owned padded batch payload.
func DecodeClosedIssuanceRequest(body []byte) (ClosedIssuanceRequest, error) {
	if len(body) != closedTerminalOperationSize || body[0] != closedIssuanceOperation {
		return ClosedIssuanceRequest{}, errors.New("closed issuance operation is invalid")
	}
	request := ClosedIssuanceRequest{Payload: append([]byte(nil), body[33:]...)}
	copy(request.Nonce[:], body[1:33])
	if request.Nonce == [32]byte{} {
		return ClosedIssuanceRequest{}, errors.New("closed issuance nonce is invalid")
	}
	return request, nil
}

// EncodeClosedIssuanceResult binds one fixed credential payload to the
// matching request nonce inside the exact 16 KiB terminal RESULT body.
func EncodeClosedIssuanceResult(nonce [32]byte, status uint8, payload []byte) ([]byte, error) {
	if nonce == [32]byte{} || status > 4 || len(payload) != closedIssuanceResultPayload {
		return nil, errors.New("closed issuance result is invalid")
	}
	body := make([]byte, closedTerminalOperationSize)
	copy(body[:32], nonce[:])
	body[32] = status
	binary.BigEndian.PutUint32(body[33:37], uint32(len(payload)))
	copy(body[37:], payload)
	return body, nil
}
