package node

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// ClosedDataJoinProfile reserves only the local durable spend
// root. Current State supplies the receiving identity, role and Carrier.
type ClosedDataJoinProfile struct {
	AdmissionRoot   string
	Certificate     tls.Certificate
	ConnectionLimit uint16
	DrainTimeout    time.Duration
}

func validateClosedDataJoinProfile(local ClosedDataJoinProfile, config runtimeConfig, snapshot dutyFacts, now time.Time) error {
	if local.AdmissionRoot == "" ||
		!filepath.IsAbs(local.AdmissionRoot) || filepath.Clean(local.AdmissionRoot) != local.AdmissionRoot ||
		local.Certificate.PrivateKey == nil || local.ConnectionLimit == 0 || local.ConnectionLimit > 16 ||
		local.DrainTimeout <= 0 || local.DrainTimeout > time.Minute || !literalNodeEndpoint(snapshot.ProbeEndpoint) ||
		route.CarrierProfile(snapshot.CarrierProfile) != route.ClosedCarrierTCP && route.CarrierProfile(snapshot.CarrierProfile) != route.ClosedCarrierQUIC {
		return errors.New("closed JOIN local reservation is incomplete")
	}
	if _, ok := closedRouteReceiver(config, snapshot, route.ClosedPurposeDataJoin, now); !ok {
		return errors.New("closed JOIN State projection is unavailable")
	}
	return nil
}

func startClosedDataJoin(config runtimeConfig, snapshot dutyFacts) (*probeServer, error) {
	local := config.ClosedDataJoin
	if err := validateClosedDataJoinProfile(local, config, snapshot, config.now()); err != nil {
		return nil, err
	}
	receiver, available := closedRouteReceiver(config, snapshot, route.ClosedPurposeDataJoin, config.now())
	if !available {
		return nil, errors.New("closed JOIN State changed before reservation")
	}
	spends, err := route.OpenClosedSpendLedger(local.AdmissionRoot, route.ClosedSpendBinding{NetworkID: receiver.NetworkID,
		ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		return nil, err
	}
	limits, err := route.NewClosedDutyLimits(config.now)
	if err != nil {
		return nil, errors.Join(err, spends.Close())
	}
	pairs, err := route.NewClosedJoinPairs(receiver, limits)
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
	running := &closedDataJoinServer{config: config, receiver: receiver, certificate: local.Certificate, listener: shared,
		pairs: pairs, spends: spends, limits: limits, capacity: make(chan struct{}, local.ConnectionLimit), cancel: cancel,
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

type closedDataJoinServer struct {
	pairs       *route.ClosedJoinPairs
	config      runtimeConfig
	receiver    route.ClosedRoleReceiver
	certificate tls.Certificate
	listener    route.ClosedSharedCarrierListener
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
	cleanupMu   sync.Mutex
	cleanupErr  error
}

func (server *closedDataJoinServer) stop() error {
	server.stopOnce.Do(func() {
		server.cancel()
		server.stopErr = server.listener.Close()
	})
	return server.stopErr
}

func (server *closedDataJoinServer) run(ctx context.Context) {
	err := server.accept(ctx)
	stopErr := server.stop()
	server.done <- err
	server.workers.Wait()
	server.pairs.Close()
	// No timeout releases roots while a child still owns a commit or reply.
	server.drainErr = errors.Join(stopErr, server.cleanupErr, server.spends.Close())
	close(server.drained)
}

func (server *closedDataJoinServer) accept(ctx context.Context) error {
	for {
		carrier, err := server.listener.Accept(ctx, 10*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if ctx.Err() != nil || carrier.Kind != route.ClosedSharedNode {
			server.closeCarrier(carrier.Connection)
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
				defer server.closeCarrier(carrier.Connection)
				server.serveOuter(ctx, carrier)
			})
		default:
			server.closeCarrier(carrier.Connection)
		}
	}
}

func (server *closedDataJoinServer) closeCarrier(connection net.Conn) {
	err := connection.Close()
	if err == nil || errors.Is(err, net.ErrClosed) {
		return
	}
	server.cleanupMu.Lock()
	server.cleanupErr = errors.Join(server.cleanupErr, err)
	server.cleanupMu.Unlock()
}
