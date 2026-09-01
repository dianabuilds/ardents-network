package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

type applicationSession struct {
	lease   *broker.ActiveSession
	receipt broker.Receipt
}

// Admit issues one local capability for the exact Broker grant. Endpoint's
// role-specific operations consume it before touching publication or a Route.
func (endpoint *endpoint) Admit(principal [32]byte, surface broker.Surface) ([32]byte, error) {
	if endpoint == nil || (surface != broker.Connection && surface != broker.Administration) {
		return [32]byte{}, errors.New("local interface surface is not granted")
	}
	return endpoint.admission.Admit(principal, surface)
}

func (endpoint *endpoint) consume(capability, principal [32]byte, surface string) (broker.Receipt, error) {
	return endpoint.admission.Consume(capability, principal, broker.Surface(surface))
}

func (endpoint *endpoint) beginApplicationSession(ctx context.Context, principal [32]byte) (*applicationSession, error) {
	if endpoint == nil || ctx == nil {
		return nil, errors.New("local Application Connection admission is unavailable")
	}
	capability, err := endpoint.admission.Admit(principal, broker.Connection)
	if err != nil {
		return nil, errors.New("local Application Connection admission is unavailable")
	}
	return endpoint.activateApplicationSession(ctx, capability, principal)
}

func (endpoint *endpoint) activateApplicationSession(ctx context.Context, capability, principal [32]byte) (*applicationSession, error) {
	if endpoint == nil || ctx == nil {
		return nil, errors.New("local Application Connection authorization is unavailable")
	}
	lease, receipt, err := endpoint.admission.Activate(ctx, capability, principal, broker.Connection)
	if err != nil {
		return nil, errors.New("local Application Connection authorization is unavailable")
	}
	return &applicationSession{lease: lease, receipt: receipt}, nil
}

func (session *applicationSession) Context() context.Context {
	if session == nil || session.lease == nil {
		return context.Background()
	}
	return session.lease.Context()
}

func (session *applicationSession) Release() {
	if session != nil && session.lease != nil {
		session.lease.Release()
	}
}

func projectReceipt(result *runtimeResult, receipt broker.Receipt) {
	result.Admission = receipt
}
