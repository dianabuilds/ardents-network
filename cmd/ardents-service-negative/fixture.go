package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"time"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

type fixture struct {
	now                         time.Time
	network, connection, admin  [32]byte
	authorityPublic             ed25519.PublicKey
	authorityPrivate            ed25519.PrivateKey
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
	endpoint, _ := serviceconn.New(serviceconn.Setup{NetworkID: value.network, BrokerID: [32]byte{4},
		AuthorityPublic: value.authorityPublic, ConnectionPrincipal: value.connection,
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
		Credential: credential, InstancePrivate: private, IntroductionAcknowledgement: [32]byte{1}, At: value.now})
}
