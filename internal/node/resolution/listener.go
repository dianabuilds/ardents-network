package resolution

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// ClosedResolutionProfile reserves only the local durable Descriptor and spend
// roots. Current State supplies the receiving identity, role and Carrier.
type ClosedResolutionProfile struct {
	Root, AdmissionRoot string
	Certificate         tls.Certificate
	ConnectionLimit     uint16
	DrainTimeout        time.Duration
}

// Config contains only the selected resolution duty's inputs. CurrentDuty and
// Authority borrow current State views at each existing admission recheck.
// VerifyAdmission retains the process host reservation for a class-1 lease.
type Config struct {
	Profile         ClosedResolutionProfile
	Snapshot        state.NodeDuty
	Authority       authority.Source
	CurrentDuty     func() (state.NodeDuty, error)
	VerifyAdmission func(route.ClosedRoleReceiver) route.ClosedAdmissionVerifier
	Now             func() time.Time
	ListenAddress   string
}

// Handle transfers supervision of the running role to Node while retaining
// ownership of its listener, workers, Descriptor store, and spend ledger.
type Handle struct {
	Done   <-chan error
	Joined <-chan struct{}
	Usage  func() (uint64, uint64, uint64)
	Stop   func()
	Drain  func(context.Context) error
}

func Validate(local ClosedResolutionProfile, source authority.Source, snapshot state.NodeDuty, now time.Time, endpointLiteral bool) error {
	if local.Root == "" || local.AdmissionRoot == "" || local.Root == local.AdmissionRoot ||
		!filepath.IsAbs(local.Root) || filepath.Clean(local.Root) != local.Root ||
		!filepath.IsAbs(local.AdmissionRoot) || filepath.Clean(local.AdmissionRoot) != local.AdmissionRoot ||
		local.Certificate.PrivateKey == nil || local.ConnectionLimit == 0 || local.ConnectionLimit > 16 ||
		local.DrainTimeout <= 0 || local.DrainTimeout > time.Minute || !endpointLiteral ||
		routecarrier.CarrierProfile(snapshot.CarrierProfile) != routecarrier.ClosedCarrierTCP && routecarrier.CarrierProfile(snapshot.CarrierProfile) != routecarrier.ClosedCarrierQUIC {
		return errors.New("closed resolution local reservation is incomplete")
	}
	if _, ok := source.Receiver(snapshot, ardp.PurposeReachability, now); !ok {
		return errors.New("closed resolution State projection is unavailable")
	}
	return nil
}

func Start(config Config) (*Handle, error) {
	if config.Authority.CurrentRoute == nil || config.CurrentDuty == nil || config.VerifyAdmission == nil || config.Now == nil {
		return nil, errors.New("closed resolution dependencies are incomplete")
	}
	local, snapshot := config.Profile, config.Snapshot
	receiver, available := config.Authority.Receiver(snapshot, ardp.PurposeReachability, config.Now())
	if !available {
		return nil, errors.New("closed resolution State changed before reservation")
	}
	spends, err := replay.Open(local.AdmissionRoot, replay.Binding{NetworkID: receiver.NetworkID,
		ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		return nil, err
	}
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: local.Root, NetworkID: receiver.NetworkID})
	if err != nil {
		return nil, errors.Join(err, spends.Close())
	}
	limits, err := route.NewClosedDutyLimits(config.Now)
	if err != nil {
		return nil, errors.Join(err, store.Close(), spends.Close())
	}
	shared, err := routecarrier.ListenClosedSharedCarrier(routecarrier.CarrierProfile(snapshot.CarrierProfile), config.ListenAddress, local.Certificate,
		func(key [32]byte) bool {
			updated, err := config.CurrentDuty()
			return err == nil && config.Authority.PeerCurrent(updated, key, config.Now())
		}, local.ConnectionLimit)
	if err != nil {
		return nil, errors.Join(err, store.Close(), spends.Close())
	}
	ctx, cancel := context.WithCancel(context.Background())
	running := &closedResolutionServer{config: config, receiver: receiver, certificate: local.Certificate, listener: shared,
		store: store, spends: spends, limits: limits, capacity: make(chan struct{}, local.ConnectionLimit), cancel: cancel,
		done: make(chan error, 1), drained: make(chan struct{})}
	go running.run(ctx)
	return &Handle{Done: running.done, Joined: running.drained, Usage: func() (uint64, uint64, uint64) {
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

type closedResolutionServer struct {
	config      Config
	receiver    route.ClosedRoleReceiver
	certificate tls.Certificate
	listener    routecarrier.ClosedSharedCarrierListener
	store       *reachability.Store
	spends      *replay.Ledger
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

func (server *closedResolutionServer) stop() error {
	server.stopOnce.Do(func() {
		server.cancel()
		server.stopErr = server.listener.Close()
	})
	return server.stopErr
}

func (server *closedResolutionServer) run(ctx context.Context) {
	err := server.accept(ctx)
	stopErr := server.stop()
	server.done <- err
	server.workers.Wait()
	// No timeout releases roots while a child still owns a commit or reply.
	server.drainErr = errors.Join(stopErr, server.store.Close(), server.spends.Close(), server.cleanupErr)
	close(server.drained)
}

func (server *closedResolutionServer) accept(ctx context.Context) error {
	for {
		carrier, err := server.listener.Accept(ctx, 10*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if routecarrier.IsClosedSharedPeerFailure(err) {
				continue
			}
			return err
		}
		if ctx.Err() != nil || carrier.Kind != routecarrier.ClosedSharedNode {
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

func (server *closedResolutionServer) closeCarrier(connection net.Conn) {
	err := connection.Close()
	if errors.Is(err, net.ErrClosed) {
		return
	}
	server.recordCleanup(err)
}

func (server *closedResolutionServer) recordCleanup(err error) {
	if err == nil {
		return
	}
	server.cleanupMu.Lock()
	server.cleanupErr = errors.Join(server.cleanupErr, err)
	server.cleanupMu.Unlock()
}
