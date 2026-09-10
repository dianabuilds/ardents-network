package credential

import (
	"errors"
)

const (
	closedTokenOutcomeMagic       = "ARDIOR01"
	closedTokenTerminalResultSize = 16 << 10
	closedTokenOutcomeSize        = closedTokenTerminalResultSize - 32 - 1 - 4
)

// EncodeClosedTokenBatchResult returns the fixed-size issuer payload for every
// outcome. Its caller puts it in the fixed 16 KiB terminal RESULT body with
// the protocol-owned nonce, status, and length fields.
func EncodeClosedTokenBatchResult(result ClosedTokenBatchResult) ([]byte, error) {
	if !validClosedTokenBatchResult(result) {
		return nil, errors.New("closed token batch result is invalid")
	}
	raw := make([]byte, closedTokenOutcomeSize)
	copy(raw[:8], closedTokenOutcomeMagic)
	raw[8] = byte(result.Status)
	raw[9] = byte(len(result.Signatures))
	offset := 10
	for _, signature := range result.Signatures {
		copy(raw[offset:offset+closedTokenBlindElementSize], signature)
		offset += closedTokenBlindElementSize
	}
	return raw, nil
}

func validClosedTokenBatchResult(result ClosedTokenBatchResult) bool {
	if result.Status < ClosedTokenIssued || result.Status > ClosedTokenUnavailable {
		return false
	}
	if result.Status != ClosedTokenIssued {
		return len(result.Signatures) == 0
	}
	if len(result.Signatures) < 1 || len(result.Signatures) > maximumClosedTokenBatch {
		return false
	}
	for _, signature := range result.Signatures {
		if len(signature) != closedTokenBlindElementSize || allZeroClosedTokenBytes(signature) {
			return false
		}
	}
	return true
}

func allZeroClosedTokenBytes(raw []byte) bool {
	for _, value := range raw {
		if value != 0 {
			return false
		}
	}
	return true
}
