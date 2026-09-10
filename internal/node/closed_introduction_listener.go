package node

import (
	"context"
	"crypto/tls"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// ClosedIntroductionProfile reserves only the local durable spend
// root. Current State supplies the receiving identity, role and Carrier.
type ClosedIntroductionProfile struct {
	AdmissionRoot   string
	Certificate     tls.Certificate
	ConnectionLimit uint16
	DrainTimeout    time.Duration
}

func validateClosedIntroductionProfile(local ClosedIntroductionProfile, config runtimeConfig, snapshot dutyFacts, now time.Time) error {
	if local.AdmissionRoot == "" ||
		!filepath.IsAbs(local.AdmissionRoot) || filepath.Clean(local.AdmissionRoot) != local.AdmissionRoot ||
		local.Certificate.PrivateKey == nil || local.ConnectionLimit == 0 || local.ConnectionLimit > 16 ||
		local.DrainTimeout <= 0 || local.DrainTimeout > time.Minute || !literalNodeEndpoint(snapshot.ProbeEndpoint) ||
		route.CarrierProfile(snapshot.CarrierProfile) != route.ClosedCarrierTCP && route.CarrierProfile(snapshot.CarrierProfile) != route.ClosedCarrierQUIC {
		return errors.New("closed Introduction local reservation is incomplete")
	}
	if _, ok := closedRouteReceiver(config, snapshot, route.ClosedPurposeIntroduction, now); !ok {
		return errors.New("closed Introduction State projection is unavailable")
	}
	return nil
}

func startClosedIntroduction(config runtimeConfig, snapshot dutyFacts) (*probeServer, error) {
	local := config.ClosedIntroduction
	if err := validateClosedIntroductionProfile(local, config, snapshot, config.now()); err != nil {
		return nil, err
	}
	receiver, available := closedRouteReceiver(config, snapshot, route.ClosedPurposeIntroduction, config.now())
	if !available {
		return nil, errors.New("closed Introduction State changed before reservation")
	}
	spends, err := route.OpenClosedSpendLedger(local.AdmissionRoot, route.ClosedSpendBinding{NetworkID: receiver.NetworkID,
		ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		return nil, err
	}
	slotFloor, err := spends.IntroductionSlots()
	if err != nil {
		return nil, errors.Join(err, spends.Close())
	}
	limits, err := route.NewClosedDutyLimits(config.now)
	if err != nil {
		return nil, errors.Join(err, spends.Close())
	}
	shared, err := route.ListenClosedSharedCarrier(route.CarrierProfile(snapshot.CarrierProfile), snapshot.ProbeEndpoint, local.Certificate,
		func(key [32]byte) bool {
			updated, err := currentFacts(config)
			return err == nil && closedSharedPeerCurrent(config, updated, key, config.now())
		}, local.ConnectionLimit)
	if err != nil {
		return nil, errors.Join(err, spends.Close())
	}
	ctx, cancel := context.WithCancel(context.Background())
	running := &closedIntroductionServer{config: config, receiver: receiver, certificate: local.Certificate, listener: shared,
		slots: make(map[[32]byte]*closedIntroductionSlot), slotFloor: slotFloor, spends: spends, limits: limits, capacity: make(chan struct{}, local.ConnectionLimit), cancel: cancel,
		done: make(chan error, 1), drained: make(chan struct{})}
	go running.run(ctx)
	return &probeServer{Done: running.done, Protect: func(bool) {}, Usage: func() (uint64, uint64, uint64) {
		active := uint64(running.active.Load())
		return active, active, 0
	}, Stop: func() { _ = running.stop() }, Drain: func(ctx context.Context) error {
		_ = running.stop()
		bounded, cancel := context.WithTimeout(ctx, local.DrainTimeout)
		defer cancel()
		select {
		case <-running.drained:
			return running.drainErr
		case <-bounded.Done():
			return bounded.Err()
		}
	}}, nil
}

type closedIntroductionServer struct {
	slotFloor   *route.ClosedIntroductionSlots
	config      runtimeConfig
	receiver    route.ClosedRoleReceiver
	certificate tls.Certificate
	listener    route.ClosedSharedCarrierListener
	slotsMu     sync.Mutex
	slots       map[[32]byte]*closedIntroductionSlot
	spends      *route.ClosedSpendLedger
	limits      *route.ClosedDutyLimits
	capacity    chan struct{}
	active      atomic.Uint32
	cancel      context.CancelFunc
	stopOnce    sync.Once
	stopErr     error
	workers     sync.WaitGroup
	done        chan error
	drained     chan struct{}
	drainErr    error
}

func (server *closedIntroductionServer) stop() error {
	server.stopOnce.Do(func() {
		server.cancel()
		server.stopErr = server.listener.Close()
	})
	return server.stopErr
}

func (server *closedIntroductionServer) run(ctx context.Context) {
	err := server.accept(ctx)
	stopErr := server.stop()
	server.done <- err
	server.workers.Wait()
	// No timeout releases roots while a child still owns a commit or reply.
	server.drainErr = errors.Join(stopErr, server.spends.Close())
	close(server.drained)
}

func (server *closedIntroductionServer) accept(ctx context.Context) error {
	for {
		carrier, err := server.listener.Accept(ctx, 10*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if ctx.Err() != nil || carrier.Kind != route.ClosedSharedNode {
			_ = carrier.Connection.Close()
			if ctx.Err() != nil {
				return nil
			}
			continue
		}
		select {
		case server.capacity <- struct{}{}:
			server.active.Add(1)
			server.workers.Go(func() {
				defer func() { <-server.capacity; server.active.Add(^uint32(0)) }()
				defer carrier.Connection.Close()
				server.serveOuter(ctx, carrier)
			})
		default:
			_ = carrier.Connection.Close()
		}
	}
}
