package node

import (
	"crypto/tls"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node/issuer"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

// ClosedIssuerProfile retains the public Node configuration name while the
// issuer role owns its local reservation and lifecycle.
type ClosedIssuerProfile = issuer.Profile

// ClosedForwardingProfile contains the local material for one closed Route
// forwarding duty. Root is exclusively owned by its current receiving duty;
// it must not share an issuer root or survive a changed duty generation.
type ClosedForwardingProfile struct {
	Root            string
	Certificate     tls.Certificate
	ConnectionLimit uint16
	DrainTimeout    time.Duration
	// HostingRoot names the installed shared provider-period ledger. It is
	// local operator configuration, never State or token material.
	HostingRoot string
	// CarrierRelayEndpoint is an optional operator-owned transparent relay
	// address for this forwarding Node's State-selected next Carrier. It cannot
	// change the selected Node identity, key, duty, profile, or TLS verification.
	CarrierRelayEndpoint string
	AdmissionTraffic     resource.HostingTraffic
	TerminationTraffic   resource.HostingTraffic
	host                 closedForwardingHost
}
