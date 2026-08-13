package servicenegative

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"time"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

type fixture struct {
	now                         time.Time
	network, connection, admin  [32]byte
	authorityPublic             ed25519.PublicKey
	authorityPrivate            ed25519.PrivateKey
	introductionPublic          ed25519.PublicKey
	introductionPrivate         ed25519.PrivateKey
	firstPrivate, secondPrivate ed25519.PrivateKey
	first, second               serviceconn.Credential
}

func newFixture() (fixture, error) {
	value := fixture{now: time.Unix(2_000_000_000, 0), network: [32]byte{1},
		connection: [32]byte{2}, admin: [32]byte{3}}
	var err error
	value.authorityPublic, value.authorityPrivate, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return value, err
	}
	value.introductionPublic, value.introductionPrivate, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return value, err
	}
	firstPublic, firstPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return value, err
	}
	secondPublic, secondPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return value, err
	}
	value.firstPrivate, value.secondPrivate = firstPrivate, secondPrivate
	value.first, err = value.issue(firstPublic, 1, value.network, value.now.Add(time.Minute))
	if err == nil {
		value.second, err = value.issue(secondPublic, 2, value.network, value.now.Add(time.Minute))
	}
	return value, err
}

func (value fixture) issue(instance ed25519.PublicKey, generation uint64, network [32]byte,
	notAfter time.Time) (serviceconn.Credential, error) {
	var authority, public [32]byte
	copy(authority[:], value.authorityPublic)
	copy(public[:], instance)
	return serviceconn.IssueCredential(value.authorityPrivate, serviceconn.Credential{
		AuthorityPublic: authority, InstancePublic: public, Generation: generation,
		NotBefore: value.now.Add(-time.Minute).Unix(), NotAfter: notAfter.Unix(),
		NetworkID: network, Capabilities: 3,
	})
}

func (value fixture) endpoint() *serviceconn.Endpoint {
	return value.endpointWithBroker([32]byte{4})
}

func (value fixture) endpointWithBroker(broker [32]byte) *serviceconn.Endpoint {
	endpoint, _ := serviceconn.New(serviceconn.Setup{NetworkID: value.network, BrokerID: broker,
		AuthorityPublic: value.authorityPublic, IntroductionPublic: value.introductionPublic,
		ConnectionPrincipal:     value.connection,
		AdministrationPrincipal: value.admin})
	return endpoint
}

func admit(ctx context.Context, endpoint *serviceconn.Endpoint, principal [32]byte,
	surface string, at time.Time) [32]byte {
	result, _ := endpoint.Do(ctx, serviceconn.Request{Action: "admit", Principal: principal, Surface: surface, At: at})
	return result.Session
}

func publish(ctx context.Context, endpoint *serviceconn.Endpoint, value fixture,
	credential serviceconn.Credential, private ed25519.PrivateKey) (serviceconn.Result, error) {
	session := admit(ctx, endpoint, value.admin, "administration", value.now)
	return endpoint.Do(ctx, serviceconn.Request{Action: "publish", Principal: value.admin, Session: session,
		Credential: credential, InstancePrivate: private,
		IntroductionAcknowledgement: value.acknowledgement(credential), At: value.now})
}

func (value fixture) acknowledgement(credential serviceconn.Credential) []byte {
	body := make([]byte, 149)
	copy(body[:4], "ASIA")
	body[4] = 1
	copy(body[5:37], credential.Target[:])
	binary.BigEndian.PutUint64(body[37:45], credential.Generation)
	binary.BigEndian.PutUint64(body[45:53], uint64(credential.NotAfter))
	copy(body[53:85], credential.NetworkID[:])
	body[85] = 4
	body[117] = 1
	message := append([]byte("ardents-h3-introduction-ack-v1\x00"), body...)
	return append(body, ed25519.Sign(value.introductionPrivate, message)...)
}
