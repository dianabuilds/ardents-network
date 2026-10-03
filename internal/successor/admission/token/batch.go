package token

import (
	"crypto/rsa"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

// Batch grammar belongs to the Admission quota contract. The holder uses that
// same grammar rather than maintaining a second validator.
type ClosedTokenBatchRequest = admission.ClosedTokenBatchRequest
type ClosedTokenBatchStatus = admission.ClosedTokenBatchStatus
type ClosedTokenBatchResult = admission.ClosedTokenBatchResult

const (
	ClosedTokenIssued           = admission.ClosedTokenIssued
	ClosedTokenExhausted        = admission.ClosedTokenExhausted
	ClosedTokenWithdrawn        = admission.ClosedTokenWithdrawn
	ClosedTokenUnavailable      = admission.ClosedTokenUnavailable
	MaximumBatchTokens          = admission.MaximumBatchTokens
	BlindSignatureSize          = admission.BlindSignatureSize
	closedTokenBatchMagic       = "ARDIBR01"
	closedTokenRequestSize      = 259
	closedTokenBlindElementSize = BlindSignatureSize
)

func closedTokenBatchTranscript(r ClosedTokenBatchRequest) []byte {
	return admission.TokenBatchTranscript(r)
}
func validClosedTokenElement(raw []byte, key *rsa.PublicKey) bool {
	return admission.ValidTokenElement(raw, key)
}
