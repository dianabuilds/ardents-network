package endpoint

import (
	"context"
	"crypto/ed25519"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

func (endpoint *endpoint) unpublish(ctx context.Context, input withdrawalRequest) (result withdrawalResult, err error) {
	receipt, consumeErr := endpoint.consume(input.Capability, input.Principal, "administration")
	if consumeErr != nil {
		return withdrawalDenied(consumeErr.Error())
	}
	result.Receipt = receipt
	if endpoint.publications == nil {
		return withdrawalFailed("service unavailable", "publisher has no publication owner", errors.New("publication root is unavailable"))
	}
	endpoint.publisherMu.Lock()
	defer endpoint.publisherMu.Unlock()
	if endpoint.publisherSession != nil {
		if err := endpoint.publisherSession.Close(); err != nil {
			return withdrawalFailed("service unavailable", "Publisher Introduction slot could not be closed", err)
		}
		endpoint.publisherSession = nil
	}
	lease, err := endpoint.publications.AcquireAt(ctx, input.At)
	if err != nil {
		if endpoint.publisherBinding != nil {
			_ = endpoint.publisherBinding.Withdraw()
			endpoint.publisherBinding = nil
		}
		return withdrawalFailed("service unavailable", "Service is not currently published", err)
	}
	current := lease.Current()
	if err := lease.Close(); err != nil {
		return withdrawalFailed("service unavailable", "current publication could not be released", err)
	}
	if err := endpoint.publications.Unpublish(ctx); err != nil {
		return withdrawalFailed("service unavailable", "Service publication could not be withdrawn", err)
	}
	if endpoint.publisherBinding != nil {
		binding := endpoint.publisherBinding
		endpoint.publisherBinding = nil
		if err := binding.Withdraw(); err != nil {
			return withdrawalFailed("service unavailable", "Service Instance binding could not be withdrawn", err)
		}
	}
	return withdrawalResult{Class: "unpublished", AuthenticatedTarget: current.Credential.Target,
		Generation: current.Credential.Generation, Receipt: receipt}, nil
}

func decodePublication(encoded []byte, authority, network [32]byte, at time.Time) (publicationCredential, error) {
	current, err := publication.Decode(encoded, ed25519.PublicKey(authority[:]), network, at)
	if err != nil {
		return publicationCredential{}, err
	}
	return current.Credential, nil
}
