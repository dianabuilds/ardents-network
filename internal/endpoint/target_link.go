package endpoint

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// Network-binding errors prevent a destination supplied for a different
// Ardents Network from entering this Endpoint's connection flow.
var (
	ErrTargetLinkNetwork   = errors.New("target link is bound to another network")
	ErrAlphaBindingNetwork = errors.New("alpha binding is bound to another network")
)

// TargetFromLink verifies that text names one Target in this Endpoint's
// configured network. It does not resolve or connect to that Target.
func (endpoint *endpoint) TargetFromLink(text string) ([32]byte, error) {
	if endpoint == nil {
		return [32]byte{}, errors.New("endpoint is required")
	}
	link, err := targetlink.Decode(text)
	if err != nil {
		return [32]byte{}, err
	}
	if link.Network != endpoint.network {
		return [32]byte{}, ErrTargetLinkNetwork
	}
	return link.Target, nil
}
