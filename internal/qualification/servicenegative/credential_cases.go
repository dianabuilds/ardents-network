package servicenegative

import (
	"context"
	"crypto/ed25519"
	"net"
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

func (value fixture) staleGenerationNewWork(ctx context.Context) bool {
	publisher := value.endpoint()
	first, err := publish(ctx, publisher, value, value.first, value.firstPrivate)
	if err != nil {
		return false
	}
	if _, err := publish(ctx, publisher, value, value.second, value.secondPrivate); err != nil {
		return false
	}
	client := value.endpointWithBroker([32]byte{8})
	clientRoute, publisherRoute := net.Pipe()
	clientApp, clientPeer := net.Pipe()
	publisherApp, publisherPeer := net.Pipe()
	defer clientPeer.Close()
	defer publisherPeer.Close()
	attempt, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	results := make(chan bool, 2)
	go func() {
		session := admit(attempt, publisher, value.connection, "connection", value.now)
		result, acceptErr := publisher.Do(attempt, serviceconn.Request{Action: "accept", Principal: value.connection,
			Session: session, Route: publisherRoute, Application: publisherApp, BytesEachDirection: 1, At: value.now})
		results <- targetFailure(result, acceptErr)
	}()
	go func() {
		session := admit(attempt, client, value.connection, "connection", value.now)
		result, connectErr := client.Do(attempt, serviceconn.Request{Action: "connect", Principal: value.connection,
			Session: session, Target: value.first.Target, Publication: first.Publication,
			Route: clientRoute, Application: clientApp, BytesEachDirection: 1, At: value.now})
		results <- targetFailure(result, connectErr)
	}()
	clientRejected, publisherRejected := <-results, <-results
	return clientRejected && publisherRejected
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

func (value fixture) notYetValid(ctx context.Context) bool {
	at := value.now.Add(-2 * time.Minute)
	endpoint := value.endpoint()
	session := admit(ctx, endpoint, value.admin, "administration", at)
	result, err := endpoint.Do(ctx, serviceconn.Request{Action: "publish", Principal: value.admin,
		Session: session, Credential: value.first, InstancePrivate: value.firstPrivate,
		IntroductionAcknowledgement: value.acknowledgement(value.first), At: at})
	return targetFailure(result, err)
}

func (value fixture) wrongCapability(ctx context.Context) bool {
	public := value.firstPrivate.Public().(ed25519.PublicKey)
	var instance [32]byte
	copy(instance[:], public)
	credential, err := serviceconn.IssueCredential(value.authorityPrivate, serviceconn.Credential{
		InstancePublic: instance, Generation: 1, NotBefore: value.now.Add(-time.Minute).Unix(),
		NotAfter: value.now.Add(time.Minute).Unix(), NetworkID: value.network, Capabilities: 1})
	if err != nil {
		return false
	}
	result, err := publish(ctx, value.endpoint(), value, credential, value.firstPrivate)
	return targetFailure(result, err)
}

func (value fixture) malformedPublication(ctx context.Context) bool {
	endpoint := value.endpoint()
	session := admit(ctx, endpoint, value.connection, "connection", value.now)
	result, err := endpoint.Do(ctx, serviceconn.Request{Action: "connect", Principal: value.connection,
		Session: session, Target: value.first.Target, Publication: []byte{1, 2, 3}, At: value.now})
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
