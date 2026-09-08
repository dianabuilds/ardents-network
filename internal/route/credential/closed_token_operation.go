package credential

import (
	"encoding/binary"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
)

const closedTokenBatchCountOffset = len(closedTokenBatchMagic) + permissionSize + 32 + 1 + 8 + 346

// IssueTerminalOperation processes only the fixed target-free issuer terminal
// operation. It neither receives a destination nor opens another lane.
func (issuer *ClosedTokenIssuer) IssueTerminalOperation(body []byte) ([]byte, error) {
	request, err := route.DecodeClosedIssuanceRequest(body)
	if err != nil {
		return nil, err
	}
	batch, err := decodeClosedTokenBatchPadded(request.Payload)
	result := ClosedTokenBatchResult{Status: ClosedTokenUnavailable}
	if err == nil {
		result = issuer.Issue(batch)
	}
	payload, encodeErr := EncodeClosedTokenBatchResult(result)
	if encodeErr != nil {
		return nil, encodeErr
	}
	return route.EncodeClosedIssuanceResult(request.Nonce, closedTokenTerminalStatus(result.Status), payload)
}

// FinalizeTerminalOperation verifies the exact nonce/result wrapper before
// exposing a complete volatile ready-token batch.
func (pending *PendingClosedTokenBatch) FinalizeTerminalOperation(nonce [32]byte, body []byte) ([][]byte, error) {
	result, err := route.DecodeClosedIssuanceResult(body, nonce)
	if err != nil || result.Status != 0 {
		if pending != nil {
			pending.Discard()
		}
		return nil, errors.New("closed issuance terminal result is unavailable")
	}
	return pending.FinalizeEncoded(result.Payload)
}

func decodeClosedTokenBatchPadded(raw []byte) ([]byte, error) {
	if len(raw) < closedTokenBatchCountOffset+2 || string(raw[:len(closedTokenBatchMagic)]) != closedTokenBatchMagic {
		return nil, errors.New("closed token batch operation is invalid")
	}
	count := int(binary.BigEndian.Uint16(raw[closedTokenBatchCountOffset : closedTokenBatchCountOffset+2]))
	if count < 1 || count > maximumClosedTokenBatch {
		return nil, errors.New("closed token batch operation count is invalid")
	}
	length := closedTokenBatchBaseSize() + count*closedTokenRequestSize
	if length > len(raw) {
		return nil, errors.New("closed token batch operation is truncated")
	}
	for _, value := range raw[length:] {
		if value != 0 {
			return nil, errors.New("closed token batch operation padding is invalid")
		}
	}
	batch := append([]byte(nil), raw[:length]...)
	if _, err := DecodeClosedTokenBatch(batch); err != nil {
		return nil, err
	}
	return batch, nil
}

func closedTokenTerminalStatus(status ClosedTokenBatchStatus) uint8 {
	switch status {
	case ClosedTokenIssued:
		return 0
	case ClosedTokenExhausted:
		return 2
	case ClosedTokenWithdrawn:
		return 4
	default:
		return 1
	}
}
