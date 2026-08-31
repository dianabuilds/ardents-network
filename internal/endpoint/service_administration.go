package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

// ServiceAdministrationConfig binds one preconfigured local Administration
// Principal to the Endpoint clock. It contains no Service or network facts.
type ServiceAdministrationConfig struct {
	Principal [32]byte
	Clock     func() time.Time
}

// ServiceAdministration is the narrow local Publisher start/withdraw owner
// shared by the headless CLI and any future authorized local Adapter.
type ServiceAdministration struct {
	endpoint  *endpoint
	principal [32]byte
	clock     func() time.Time
}

// OpenServiceAdministration creates no authority. Each operation receives a
// fresh volatile capability from the Endpoint's preconfigured local grant.
func (endpoint *endpoint) OpenServiceAdministration(config ServiceAdministrationConfig) (*ServiceAdministration, error) {
	if endpoint == nil || config.Principal == [32]byte{} {
		return nil, errors.New("Service Administration input is incomplete")
	}
	clock := config.Clock
	if clock == nil {
		clock = time.Now
	}
	return &ServiceAdministration{endpoint: endpoint, principal: config.Principal, clock: clock}, nil
}

// Publish starts the one Endpoint-owned Instance/publication/slot transaction.
func (owner *ServiceAdministration) Publish(ctx context.Context) error {
	if owner == nil || owner.endpoint == nil || ctx == nil {
		return errors.New("Service Administration is unavailable")
	}
	capability, err := owner.endpoint.Admit(owner.principal, broker.Administration)
	if err != nil {
		return err
	}
	result, err := owner.endpoint.StartPublisher(ctx, PublisherStartRequest{Principal: owner.principal, Capability: capability,
		At: owner.clock().UTC()})
	if err == nil && result.Class != "published" {
		err = errors.New("Publisher start returned a non-published result")
	}
	return err
}

// Withdraw closes the live slot and publication before terminally withdrawing
// the Endpoint-owned Instance binding.
func (owner *ServiceAdministration) Withdraw(ctx context.Context) error {
	if owner == nil || owner.endpoint == nil || ctx == nil {
		return errors.New("Service Administration is unavailable")
	}
	capability, err := owner.endpoint.Admit(owner.principal, broker.Administration)
	if err != nil {
		return err
	}
	result, err := owner.endpoint.Withdraw(ctx, WithdrawalRequest{Principal: owner.principal, Capability: capability,
		At: owner.clock().UTC()})
	if err == nil && result.Class != "unpublished" {
		err = errors.New("Publisher withdrawal returned a non-withdrawn result")
	}
	return err
}

var _ administration.Interface = (*ServiceAdministration)(nil)
