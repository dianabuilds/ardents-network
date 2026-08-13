package main

import (
	"context"
	"crypto/ed25519"
	"time"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func (value fixture) wrongTarget(ctx context.Context) bool {
	published, err := publish(ctx, value.endpoint(), value, value.first, value.firstPrivate)
	if err != nil {
		return false
	}
	client := value.endpoint()
	session := admit(ctx, client, value.connection, "connection", value.now)
	result, err := client.Do(ctx, serviceconn.Request{Action: "connect", Principal: value.connection,
		Session: session, Target: [32]byte{44}, Publication: published.Publication, At: value.now})
	return targetFailure(result, err)
}

func (value fixture) wrongKey(ctx context.Context) bool {
	result, err := publish(ctx, value.endpoint(), value, value.first, value.secondPrivate)
	return targetFailure(result, err)
}

func (value fixture) expired(ctx context.Context) bool {
	public := value.firstPrivate.Public().(ed25519.PublicKey)
	credential, err := value.issue(public, 1, value.network, value.now)
	if err != nil {
		return false
	}
	result, err := publish(ctx, value.endpoint(), value, credential, value.firstPrivate)
	return targetFailure(result, err)
}

func (value fixture) wrongNetwork(ctx context.Context) bool {
	public := value.firstPrivate.Public().(ed25519.PublicKey)
	credential, err := value.issue(public, 1, [32]byte{88}, value.now.Add(time.Minute))
	if err != nil {
		return false
	}
	result, err := publish(ctx, value.endpoint(), value, credential, value.firstPrivate)
	return targetFailure(result, err)
}

func (value fixture) staleGeneration(ctx context.Context) bool {
	endpoint := value.endpoint()
	if _, err := publish(ctx, endpoint, value, value.second, value.secondPrivate); err != nil {
		return false
	}
	result, err := publish(ctx, endpoint, value, value.first, value.firstPrivate)
	return targetFailure(result, err)
}

func (value fixture) sameGenerationConflict(ctx context.Context) bool {
	endpoint := value.endpoint()
	if _, err := publish(ctx, endpoint, value, value.first, value.firstPrivate); err != nil {
		return false
	}
	public := value.secondPrivate.Public().(ed25519.PublicKey)
	conflict, err := value.issue(public, 1, value.network, value.now.Add(time.Minute))
	if err != nil {
		return false
	}
	result, err := publish(ctx, endpoint, value, conflict, value.secondPrivate)
	return targetFailure(result, err)
}
