package runtime

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

// AdministrationRequest retains a fresh local admission for Link/Withdraw.
// It has no Job, worker, Publication identity or Service byte authority.
type AdministrationRequest struct {
	self    *AdministrationRequest
	session *execution.Session
}

// AdmitAdministration consumes the original owner's exact Principal grant.
// Snapshot publication separately admits its qualified Publisher invocation.
func (owner *Owner) AdmitAdministration(ctx context.Context, principal [32]byte) (*AdministrationRequest, error) {
	if owner == nil || owner.authority == nil || owner.supervisor == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("local Administration request is unavailable")
	}
	capability, err := owner.authority.Admit(principal, execution.Administration)
	if err != nil {
		return nil, err
	}
	session, err := owner.supervisor.Activate(ctx, capability, principal, execution.Administration)
	if err != nil {
		return nil, err
	}
	request := &AdministrationRequest{session: session}
	request.self = request
	if err := request.Check(); err != nil {
		return nil, errors.Join(err, request.Close())
	}
	return request, nil
}

func (request *AdministrationRequest) Check() error {
	if request == nil || request.self != request || request.session == nil {
		return errors.New("original Administration request is unavailable")
	}
	return request.session.Check()
}

func (request *AdministrationRequest) Context() context.Context {
	if request == nil || request.self != request || request.session == nil {
		return nil
	}
	return request.session.Context()
}

func (request *AdministrationRequest) Close() error {
	if request == nil {
		return nil
	}
	if request.self != request || request.session == nil {
		return errors.New("original Administration request is unavailable")
	}
	return request.session.Close()
}
