package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	alphaprivate "github.com/dianabuilds/ardents-network/internal/naming/alpha/private"
)

// ResolveAlpha accepts only the explicit alpha destination form and delegates
// one resolution attempt to the caller-provisioned private role Client. It
// verifies that the returned binding belongs to this Endpoint's Network before
// any later Service Connection may use it.
func (endpoint *endpoint) ResolveAlpha(ctx context.Context, resolver *alphaprivate.Client, text string, at time.Time) (alpha.Binding, error) {
	if endpoint == nil || resolver == nil || ctx == nil {
		return alpha.Binding{}, errors.New("alpha resolution input is incomplete")
	}
	link, err := alpha.ParseServiceLink(text)
	if err != nil {
		return alpha.Binding{}, err
	}
	binding, err := resolver.Resolve(ctx, link, at)
	if err != nil {
		return alpha.Binding{}, err
	}
	if binding.Network() != endpoint.network {
		return alpha.Binding{}, ErrAlphaBindingNetwork
	}
	return binding, nil
}

// ResolveAcceptedAlpha resolves one explicit alpha Service Link from an
// Endpoint-local corpus floor that was accepted by the separate alpha-control
// path. It performs no network request and never makes a corpus accepted: the
// caller must supply the already-owned persistent floor.
func (endpoint *endpoint) ResolveAcceptedAlpha(floor *alpha.PersistentFloor, text string, at time.Time) (alpha.Binding, error) {
	if endpoint == nil || floor == nil || at.IsZero() {
		return alpha.Binding{}, errors.New("accepted alpha resolution input is incomplete")
	}
	link, err := alpha.ParseServiceLink(text)
	if err != nil {
		return alpha.Binding{}, err
	}
	corpus, err := floor.Current()
	if err != nil {
		return alpha.Binding{}, err
	}
	binding, err := corpus.Resolve(link, at)
	if err != nil {
		return alpha.Binding{}, err
	}
	if binding.Network() != endpoint.network {
		return alpha.Binding{}, ErrAlphaBindingNetwork
	}
	return binding, nil
}
