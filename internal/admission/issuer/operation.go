package issuer

import (
	admissiontoken "github.com/dianabuilds/ardents-network/internal/admission/token"

	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

// IssueTerminalOperation processes only the fixed target-free issuer terminal
// operation. It neither receives a destination nor opens another lane.
func (issuer *ClosedTokenIssuer) IssueTerminalOperation(body []byte) ([]byte, error) {
	return issuer.issueTerminalOperation(body, closedIssuanceBootstrap)
}

func (issuer *ClosedTokenIssuer) issueTerminalOperation(body []byte, kind closedIssuanceKind) ([]byte, error) {
	request, err := terminal.DecodeIssuanceRequest(body)
	if err != nil {
		return nil, err
	}
	batch, err := admissiontoken.DecodePaddedBatch(request.Payload)
	result := admissiontoken.ClosedTokenBatchResult{Status: admissiontoken.ClosedTokenUnavailable}
	if err == nil {
		result = issuer.issue(batch, kind)
	}
	payload, encodeErr := admissiontoken.EncodeClosedTokenBatchResult(result)
	if encodeErr != nil {
		return nil, encodeErr
	}
	return terminal.EncodeIssuanceResult(request.Nonce, closedTokenTerminalStatus(result.Status), payload)
}

func closedTokenTerminalStatus(status admissiontoken.ClosedTokenBatchStatus) uint8 {
	switch status {
	case admissiontoken.ClosedTokenIssued:
		return 0
	case admissiontoken.ClosedTokenExhausted:
		return 2
	case admissiontoken.ClosedTokenWithdrawn:
		return 4
	default:
		return 1
	}
}
