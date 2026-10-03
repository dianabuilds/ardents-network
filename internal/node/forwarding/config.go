package forwarding

import (
	"context"
	"crypto/tls"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	hostingbudget "github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// Profile contains only the local resources and limits of a forwarding duty.
type Profile struct {
	Root                 string
	Certificate          tls.Certificate
	ConnectionLimit      uint16
	DrainTimeout         time.Duration
	CarrierRelayEndpoint string
}

// Host is the forwarding duty's lease of the shared provider-period ledger.
// Node opens it and supplies the class-2 reservation policy; forwarding closes
// the lease after its workers and sessions have joined.
type Host interface {
	Sample(context.Context, time.Duration) (hostingbudget.Sample, error)
	Close() error
}

// Config borrows current State and Node's admission policy for this duty.
type Config struct {
	Profile         Profile
	Snapshot        state.NodeDuty
	Receiver        route.ClosedRoleReceiver
	ListenAddress   string
	Authority       authority.Source
	CurrentDuty     func() (state.NodeDuty, error)
	VerifyAdmission func(route.ClosedRoleReceiver) route.ClosedAdmissionVerifier
	Replenish       func(route.ClosedRoleReceiver, *spending.Ledger) route.ClosedForwardingReplenisher
	LiteralEndpoint func(string) bool
	Host            Host
	Now             func() time.Time
}

// Handle transfers process supervision without transferring role resources.
type Handle struct {
	Done   <-chan error
	Joined <-chan struct{}
	Usage  func() (uint64, uint64, uint64)
	Stop   func()
	Drain  func(context.Context) error
}

func Start(config Config) (*Handle, error) {
	local, receiver, host := config.Profile, config.Receiver, config.Host
	if host == nil {
		return nil, errors.New("closed forwarding host allowance is unavailable")
	}
	if config.Authority.CurrentRoute == nil || config.Authority.CurrentProfile == nil || config.CurrentDuty == nil || config.VerifyAdmission == nil ||
		config.Replenish == nil || config.LiteralEndpoint == nil || config.Now == nil {
		return nil, errors.Join(errors.New("closed forwarding dependencies are incomplete"), host.Close())
	}
	receiving, err := openReceivingResources(local.Root, spending.Binding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest,
		ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration}, config.Now)
	if err != nil {
		return nil, errors.Join(err, host.Close())
	}
	pool, err := carrier.NewClosedCarrierPool(config.Now)
	if err != nil {
		return nil, errors.Join(err, receiving.Close(), host.Close())
	}
	shared, err := carrier.ListenClosedSharedCarrier(carrier.CarrierProfile(config.Snapshot.CarrierProfile), config.ListenAddress, local.Certificate,
		func(key [32]byte) bool {
			updated, readErr := config.CurrentDuty()
			return readErr == nil && config.Authority.PeerCurrent(updated, key, config.Now())
		}, local.ConnectionLimit)
	if err != nil {
		return nil, errors.Join(err, pool.Close(), receiving.Close(), host.Close())
	}
	dependencies := dependencies{current: config.CurrentDuty, authority: config.Authority, verify: config.VerifyAdmission,
		replenish: config.Replenish, relayEndpoint: local.CarrierRelayEndpoint, literalEndpoint: config.LiteralEndpoint}
	running := newServerWithHost(dependencies, local.Certificate, shared, receiving, pool, host, local.ConnectionLimit, config.Now)
	return &Handle{Done: running.Done(), Joined: running.drained, Usage: func() (uint64, uint64, uint64) {
		active := uint64(running.Active())
		return active, active, 0
	}, Stop: func() { _ = running.Stop() }, Drain: func(ctx context.Context) error {
		drain, cancel := context.WithTimeout(ctx, local.DrainTimeout)
		defer cancel()
		return running.Drain(drain)
	}}, nil
}

type dependencies struct {
	current         func() (state.NodeDuty, error)
	authority       authority.Source
	verify          func(route.ClosedRoleReceiver) route.ClosedAdmissionVerifier
	replenish       func(route.ClosedRoleReceiver, *spending.Ledger) route.ClosedForwardingReplenisher
	relayEndpoint   string
	literalEndpoint func(string) bool
}
