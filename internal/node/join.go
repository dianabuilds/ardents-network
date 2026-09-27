package node

import (
	"context"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	nodehosting "github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/node/join"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
)

// ClosedDataJoinProfile retains the Node configuration name while JOIN owns
// its listener, spend ledger, pair set and Hosting handle.
type ClosedDataJoinProfile = join.Profile

func validateClosedDataJoinProfile(local ClosedDataJoinProfile, config runtimeConfig, snapshot state.NodeDuty, now time.Time) error {
	return join.Validate(local, nodeAuthority(config), snapshot, now, literalNodeEndpoint(snapshot.ProbeEndpoint))
}

func startClosedDataJoin(config runtimeConfig, snapshot state.NodeDuty) (*dutyHandle, error) {
	if err := validateClosedDataJoinProfile(config.ClosedDataJoin, config, snapshot, config.now()); err != nil {
		return nil, err
	}
	listen, err := closedListenAddress(snapshot.ProbeEndpoint, config.ClosedListenOverride)
	if err != nil {
		return nil, err
	}
	role, err := join.Start(join.Config{Profile: config.ClosedDataJoin, Snapshot: snapshot,
		Authority: nodeAuthority(config), CurrentDuty: func() (state.NodeDuty, error) { return currentFacts(config) },
		Now: config.now, ListenAddress: listen, OpenHost: func(root string) (join.Host, error) {
			host, err := nodehosting.Open(root)
			if err != nil {
				return nil, err
			}
			return joinHosting{host: host, source: nodeAuthority(config), now: config.now}, nil
		}})
	if err != nil {
		return nil, err
	}
	return &dutyHandle{Done: role.Done, Protect: func(bool) {}, Usage: role.Usage, Stop: role.Stop, Drain: role.Drain}, nil
}

// joinHosting keeps the shared provider policy in Node while the JOIN role
// owns this particular handle and its close after child work has joined.
type joinHosting struct {
	host   closedHostingHandle
	source authority.Source
	now    func() time.Time
}

func (hosting joinHosting) Sample(ctx context.Context, age time.Duration) (resource.HostingSample, error) {
	return hosting.host.Sample(ctx, age)
}

func (hosting joinHosting) AdmissionVerifier(receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
	work, termination := closedJoinHostingEnvelope()
	return nodehosting.AdmissionVerifier(hosting.source, hosting.now, receiver, hosting.host, work, termination)
}

func (hosting joinHosting) Replenisher(receiver route.ClosedRoleReceiver, spends *replay.Ledger) route.ClosedForwardingReplenisher {
	work, termination := closedJoinHostingEnvelope()
	return nodehosting.Replenisher(hosting.source, hosting.now, receiver, hosting.host, spends, work, termination)
}

func (hosting joinHosting) Close() error { return hosting.host.Close() }

// The envelope includes both directions and transport/control overhead; the
// installed policy chooses which directions the actual provider charges.
func closedJoinHostingEnvelope() (resource.HostingTraffic, resource.HostingTraffic) {
	return resource.HostingTraffic{Tx: 64 << 20, Rx: 64 << 20}, resource.HostingTraffic{Tx: 1 << 20, Rx: 1 << 20}
}
