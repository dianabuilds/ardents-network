package node

import (
	"crypto/tls"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	nodeforwarding "github.com/dianabuilds/ardents-network/internal/node/forwarding"
	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
)

// ClosedForwardingProfile contains the local material for one closed Route
// forwarding duty. Its receiving root is exclusive to that duty generation.
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
	host                 closedHostingHandle
}

// startClosedForwarding composes the selected role and transfers the Host lease
// to the forwarding owner after validating the local process reservation.
func startClosedForwarding(local ClosedForwardingProfile, inputs roleInputs, snapshot state.NodeDuty) (*dutyHandle, error) {
	if err := validateClosedForwardingProfile(local, inputs.authority, snapshot, inputs.now()); err != nil {
		return nil, err
	}
	listen, err := closedListenAddress(snapshot.ProbeEndpoint, inputs.listenOverride)
	if err != nil {
		return nil, err
	}
	receiver, available := inputs.authority.Receiver(snapshot, ardp.PurposeForwarding, inputs.now())
	if !available {
		return nil, errors.New("closed forwarding receiver is unavailable")
	}
	host := local.host
	if host == nil {
		var err error
		host, err = hosting.Open(local.HostingRoot)
		if err != nil {
			return nil, err
		}
	}
	source := inputs.authority
	running, err := nodeforwarding.Start(nodeforwarding.Config{
		Profile: nodeforwarding.Profile{Root: local.Root, Certificate: local.Certificate, ConnectionLimit: local.ConnectionLimit,
			DrainTimeout: local.DrainTimeout, CarrierRelayEndpoint: local.CarrierRelayEndpoint},
		Snapshot: snapshot, Receiver: receiver, ListenAddress: listen, Authority: source,
		CurrentDuty: inputs.currentDuty,
		VerifyAdmission: func(receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
			return hosting.AdmissionVerifier(source, inputs.now, receiver, host, local.AdmissionTraffic, local.TerminationTraffic)
		},
		Replenish: func(receiver route.ClosedRoleReceiver, spends *replay.Ledger) route.ClosedForwardingReplenisher {
			return hosting.Replenisher(source, inputs.now, receiver, host, spends, local.AdmissionTraffic, local.TerminationTraffic)
		},
		LiteralEndpoint: literalNodeEndpoint, Host: host, Now: inputs.now,
	})
	if err != nil {
		return nil, err
	}
	return &dutyHandle{Done: running.Done, Joined: running.Joined, Protect: func(bool) {}, Usage: running.Usage, Stop: running.Stop, Drain: running.Drain}, nil
}

func validateClosedForwardingProfile(local ClosedForwardingProfile, source authority.Source, snapshot state.NodeDuty, now time.Time) error {
	if local.Root == "" || !filepath.IsAbs(local.Root) || filepath.Clean(local.Root) != local.Root || local.Certificate.PrivateKey == nil ||
		local.ConnectionLimit == 0 || local.ConnectionLimit > 16 || local.DrainTimeout <= 0 || local.DrainTimeout > time.Minute ||
		local.host == nil && (local.HostingRoot == "" || !filepath.IsAbs(local.HostingRoot) || filepath.Clean(local.HostingRoot) != local.HostingRoot) ||
		local.AdmissionTraffic.Tx == 0 && local.AdmissionTraffic.Rx == 0 || local.TerminationTraffic.Tx == 0 && local.TerminationTraffic.Rx == 0 ||
		!literalNodeEndpoint(snapshot.ProbeEndpoint) || (routecarrier.CarrierProfile(snapshot.CarrierProfile) != routecarrier.ClosedCarrierTCP && routecarrier.CarrierProfile(snapshot.CarrierProfile) != routecarrier.ClosedCarrierQUIC) {
		return errors.New("closed forwarding local profile is incomplete")
	}
	if _, available := source.Receiver(snapshot, ardp.PurposeForwarding, now); !available {
		return errors.New("closed forwarding State profile is unavailable")
	}
	return nil
}
