package credential

import "errors"

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

// DecodeClosedTokenBatchResult accepts only one fixed 16 KiB result and
// requires canonical zero padding to avoid a secondary response vocabulary.
func DecodeClosedTokenBatchResult(raw []byte) (ClosedTokenBatchResult, error) {
	if len(raw) != closedTokenOutcomeSize || string(raw[:8]) != closedTokenOutcomeMagic {
		return ClosedTokenBatchResult{}, errors.New("closed token batch result framing is invalid")
	}
	result := ClosedTokenBatchResult{Status: ClosedTokenBatchStatus(raw[8])}
	count := int(raw[9])
	if result.Status == ClosedTokenIssued && (count < 1 || count > maximumClosedTokenBatch) || result.Status != ClosedTokenIssued && count != 0 {
		return ClosedTokenBatchResult{}, errors.New("closed token batch result count is invalid")
	}
	offset := 10
	if offset+count*closedTokenBlindElementSize > len(raw) {
		return ClosedTokenBatchResult{}, errors.New("closed token batch result is truncated")
	}
	for index := 0; index < count; index++ {
		signature := append([]byte(nil), raw[offset:offset+closedTokenBlindElementSize]...)
		if allZeroClosedTokenBytes(signature) {
			return ClosedTokenBatchResult{}, errors.New("closed token batch signature is invalid")
		}
		result.Signatures = append(result.Signatures, signature)
		offset += closedTokenBlindElementSize
	}
	for _, value := range raw[offset:] {
		if value != 0 {
			return ClosedTokenBatchResult{}, errors.New("closed token batch result padding is invalid")
		}
	}
	if !validClosedTokenBatchResult(result) {
		return ClosedTokenBatchResult{}, errors.New("closed token batch result is invalid")
	}
	return result, nil
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
