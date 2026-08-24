package endpoint

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// ErrTargetLinkNetwork reports a link that belongs to another Ardents network.
var ErrTargetLinkNetwork = errors.New("target link is bound to another network")

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
