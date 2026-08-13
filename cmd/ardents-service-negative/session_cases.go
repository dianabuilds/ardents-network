package main

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func (value fixture) sessionReplay(ctx context.Context) bool {
	endpoint := value.endpoint()
	session := admit(ctx, endpoint, value.connection, "connection", value.now)
	_, _ = endpoint.Do(ctx, serviceconn.Request{Action: "connect", Principal: value.connection,
		Session: session, Target: value.first.Target, At: value.now})
	result, err := endpoint.Do(ctx, serviceconn.Request{Action: "connect", Principal: value.connection,
		Session: session, Target: value.first.Target, At: value.now})
	return denied(result, err)
}

func (value fixture) principalSubstitution(ctx context.Context) bool {
	endpoint := value.endpoint()
	session := admit(ctx, endpoint, value.connection, "connection", value.now)
	result, err := endpoint.Do(ctx, serviceconn.Request{Action: "connect", Principal: [32]byte{99},
		Session: session, Target: value.first.Target, At: value.now})
	return denied(result, err)
}

func (value fixture) restartReuse(ctx context.Context) bool {
	session := admit(ctx, value.endpoint(), value.connection, "connection", value.now)
	result, err := value.endpoint().Do(ctx, serviceconn.Request{Action: "connect", Principal: value.connection,
		Session: session, Target: value.first.Target, At: value.now})
	return denied(result, err)
}

func (value fixture) connectionAdministration(ctx context.Context) bool {
	endpoint := value.endpoint()
	session := admit(ctx, endpoint, value.connection, "connection", value.now)
	result, err := endpoint.Do(ctx, serviceconn.Request{Action: "publish", Principal: value.connection,
		Session: session, Credential: value.first, InstancePrivate: value.firstPrivate,
		IntroductionAcknowledgement: value.acknowledgement(value.first), At: value.now})
	return denied(result, err)
}

func (value fixture) credentialOnly(ctx context.Context) bool {
	result, err := value.endpoint().Do(ctx, serviceconn.Request{Action: "publish", Principal: value.admin,
		Credential: value.first, InstancePrivate: value.firstPrivate,
		IntroductionAcknowledgement: value.acknowledgement(value.first), At: value.now})
	return denied(result, err)
}

func (value fixture) administrationConnection(ctx context.Context) bool {
	endpoint := value.endpoint()
	session := admit(ctx, endpoint, value.admin, "administration", value.now)
	result, err := endpoint.Do(ctx, serviceconn.Request{Action: "connect", Principal: value.admin,
		Session: session, Target: value.first.Target, At: value.now})
	return denied(result, err)
}

func (value fixture) forbiddenAdministrationAction(ctx context.Context, action string) bool {
	result, err := value.endpoint().Do(ctx, serviceconn.Request{Action: action, Principal: value.admin, At: value.now})
	return denied(result, err)
}
