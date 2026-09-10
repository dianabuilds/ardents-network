//go:build linux

package route

import (
	"context"
	"crypto/rand"
	"errors"
)

// ExchangeIssuer performs one ordinary issuance through this admitted prefix.
// State fixes the sole issuer; Endpoint supplies its private batch and presenter.
func (prefix *ClosedSourcePrefix) ExchangeIssuer(ctx context.Context, present ClosedTokenPresenter, batch []byte) (ClosedIssuanceExchangeResult, error) {
	var result ClosedIssuanceExchangeResult
	if prefix == nil {
		return result, errors.New("closed source issuer owner unavailable")
	}
	if _, err := rand.Read(result.Nonce[:]); err != nil {
		return ClosedIssuanceExchangeResult{}, err
	}
	operation, err := EncodeClosedIssuanceRequest(result.Nonce, batch)
	if err != nil {
		return ClosedIssuanceExchangeResult{}, err
	}
	defer clear(operation)
	result.Body, err = prefix.exchangeControl(ctx, ClosedPurposeIssuer, prefix.plan.peers[2], present, operation)
	if err == nil {
		_, err = DecodeClosedIssuanceResult(result.Body, result.Nonce)
	}
	if err != nil {
		clear(result.Body)
		return ClosedIssuanceExchangeResult{}, err
	}
	return result, nil
}
