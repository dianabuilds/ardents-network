package node

import (
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
)

// closedForwardingDependencies is the process-to-role boundary. The forwarding
// owner borrows current authenticated State and admission policy without
// receiving the process configuration or owning the shared Hosting ledger.
type closedForwardingDependencies struct {
	current         func() (state.NodeDuty, error)
	authority       authority.Source
	verify          func(route.ClosedRoleReceiver) route.ClosedAdmissionVerifier
	replenish       func(route.ClosedRoleReceiver, *replay.Ledger) route.ClosedForwardingReplenisher
	relayEndpoint   string
	literalEndpoint func(string) bool
}

func forwardingDependencies(config runtimeConfig, host closedForwardingHost) closedForwardingDependencies {
	source := nodeAuthority(config)
	return closedForwardingDependencies{
		current:   func() (state.NodeDuty, error) { return currentFacts(config) },
		authority: source,
		verify: func(receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
			return closedForwardingAdmissionVerifier(source, config.now, receiver, host, config.ClosedForwarding)
		},
		replenish: func(receiver route.ClosedRoleReceiver, spends *replay.Ledger) route.ClosedForwardingReplenisher {
			return closedForwardingReplenisher(source, config.now, receiver, host, spends, config.ClosedForwarding)
		},
		relayEndpoint:   config.ClosedForwarding.CarrierRelayEndpoint,
		literalEndpoint: literalNodeEndpoint,
	}
}
