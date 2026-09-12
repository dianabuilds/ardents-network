//go:build linux

package credential

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/route"
)

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
