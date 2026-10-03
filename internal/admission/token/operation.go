package token

import (
	"encoding/binary"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/admission"
)

const closedTokenBatchCountOffset = len(closedTokenBatchMagic) + admission.PermissionSize + 32 + 1 + 8 + 346

// DecodePaddedBatch admits canonical zero-padded terminal request payloads.
func DecodePaddedBatch(raw []byte) ([]byte, error) {
	if len(raw) < closedTokenBatchCountOffset+2 || string(raw[:len(closedTokenBatchMagic)]) != closedTokenBatchMagic {
		return nil, errors.New("closed token batch operation is invalid")
	}
	count := int(binary.BigEndian.Uint16(raw[closedTokenBatchCountOffset : closedTokenBatchCountOffset+2]))
	if count < 1 || count > MaximumBatchTokens {
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
