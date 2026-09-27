package node

import "github.com/dianabuilds/ardents-network/internal/node/forwarding"

func closedCarrierDialAddress(advertised, relay string) (string, error) {
	return forwarding.DialAddress(advertised, relay)
}

func validClosedCarrierEndpoint(endpoint string) bool {
	return forwarding.ValidCarrierEndpoint(endpoint)
}
