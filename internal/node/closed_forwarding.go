package node

import (
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	nodeforwarding "github.com/dianabuilds/ardents-network/internal/node/forwarding"
	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
)

// startClosedForwarding composes the selected role and transfers the Host lease
// to the forwarding owner after validating the local process reservation.
func startClosedForwarding(config runtimeConfig, snapshot state.NodeDuty) (*dutyHandle, error) {
	local := config.ClosedForwarding
	if err := validateClosedForwardingProfile(local, config, snapshot, config.now()); err != nil {
		return nil, err
	}
	listen, err := closedListenAddress(snapshot.ProbeEndpoint, config.ClosedListenOverride)
	if err != nil {
		return nil, err
	}
	receiver, available := closedRouteReceiver(config, snapshot, ardp.PurposeForwarding, config.now())
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
	source := nodeAuthority(config)
	running, err := nodeforwarding.Start(nodeforwarding.Config{
		Profile: nodeforwarding.Profile{Root: local.Root, Certificate: local.Certificate, ConnectionLimit: local.ConnectionLimit,
			DrainTimeout: local.DrainTimeout, CarrierRelayEndpoint: local.CarrierRelayEndpoint},
		Snapshot: snapshot, Receiver: receiver, ListenAddress: listen, Authority: source,
		CurrentDuty: func() (state.NodeDuty, error) { return currentFacts(config) },
		VerifyAdmission: func(receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
			return hosting.AdmissionVerifier(source, config.now, receiver, host, local.AdmissionTraffic, local.TerminationTraffic)
		},
		Replenish: func(receiver route.ClosedRoleReceiver, spends *replay.Ledger) route.ClosedForwardingReplenisher {
			return hosting.Replenisher(source, config.now, receiver, host, spends, local.AdmissionTraffic, local.TerminationTraffic)
		},
		LiteralEndpoint: literalNodeEndpoint, Host: host, Now: config.now,
	})
	if err != nil {
		return nil, err
	}
	return &dutyHandle{Done: running.Done, Protect: func(bool) {}, Usage: running.Usage, Stop: running.Stop, Drain: running.Drain}, nil
}

func validateClosedForwardingProfile(local ClosedForwardingProfile, config runtimeConfig, snapshot state.NodeDuty, now time.Time) error {
	if local.Root == "" || !filepath.IsAbs(local.Root) || filepath.Clean(local.Root) != local.Root || local.Certificate.PrivateKey == nil ||
		local.ConnectionLimit == 0 || local.ConnectionLimit > 16 || local.DrainTimeout <= 0 || local.DrainTimeout > time.Minute ||
		local.host == nil && (local.HostingRoot == "" || !filepath.IsAbs(local.HostingRoot) || filepath.Clean(local.HostingRoot) != local.HostingRoot) ||
		local.AdmissionTraffic.Tx == 0 && local.AdmissionTraffic.Rx == 0 || local.TerminationTraffic.Tx == 0 && local.TerminationTraffic.Rx == 0 ||
		!literalNodeEndpoint(snapshot.ProbeEndpoint) || (routecarrier.CarrierProfile(snapshot.CarrierProfile) != routecarrier.ClosedCarrierTCP && routecarrier.CarrierProfile(snapshot.CarrierProfile) != routecarrier.ClosedCarrierQUIC) {
		return errors.New("closed forwarding local profile is incomplete")
	}
	if _, available := closedRouteReceiver(config, snapshot, ardp.PurposeForwarding, now); !available {
		return errors.New("closed forwarding State profile is unavailable")
	}
	return nil
}
