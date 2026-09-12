//go:build linux

package credential

import (
	"errors"
)

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
