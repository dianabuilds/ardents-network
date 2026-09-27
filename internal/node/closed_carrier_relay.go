package node

import "github.com/dianabuilds/ardents-network/internal/node/forwarding"

func validClosedCarrierEndpoint(endpoint string) bool {
	return forwarding.ValidCarrierEndpoint(endpoint)
}
