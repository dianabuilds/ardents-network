package endpoint

import (
	"errors"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

const retiredAlphaServiceLinkPrefix = "ardents-alpha://"

// Destination admission errors prevent retired or foreign-network input from
// entering this Endpoint's connection flow.
var (
	ErrTargetLinkNetwork       = errors.New("target link is bound to another network")
	ErrAlphaDestinationRetired = errors.New("alpha service link is retired")
)

// TargetFromLink verifies that text names one Target in this Endpoint's
// configured network. It does not resolve or connect to that Target.
func (endpoint *endpoint) TargetFromLink(text string) ([32]byte, error) {
	if endpoint == nil {
		return [32]byte{}, errors.New("endpoint is required")
	}
	if strings.HasPrefix(text, retiredAlphaServiceLinkPrefix) {
		return [32]byte{}, ErrAlphaDestinationRetired
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
