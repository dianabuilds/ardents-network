package join

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	hostingbudget "github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// Profile reserves the local spend and Hosting roots. Current State supplies
// the receiving identity, role and Carrier.
type Profile struct {
	HostingRoot     string
	AdmissionRoot   string
	Certificate     tls.Certificate
	ConnectionLimit uint16
	DrainTimeout    time.Duration
}

// Host is the role's lease of the shared provider period. Node supplies the
// class-2 Hosting policy while JOIN owns this handle and its late close.
type Host interface {
	Sample(context.Context, time.Duration) (hostingbudget.Sample, error)
	AdmissionVerifier(route.ClosedRoleReceiver) route.ClosedAdmissionVerifier
	Replenisher(route.ClosedRoleReceiver, *spending.Ledger) route.ClosedForwardingReplenisher
	Close() error
}

// Config contains only the selected JOIN duty and its borrowed dependencies.
type Config struct {
	Profile       Profile
	Snapshot      state.NodeDuty
	Authority     authority.Source
	CurrentDuty   func() (state.NodeDuty, error)
	Now           func() time.Time
	ListenAddress string
	OpenHost      func(string) (Host, error)
}

// Handle transfers process supervision without transferring role resources.
type Handle struct {
	Done   <-chan error
	Joined <-chan struct{}
	Usage  func() (uint64, uint64, uint64)
	Stop   func()
	Drain  func(context.Context) error
}

func Validate(local Profile, source authority.Source, snapshot state.NodeDuty, now time.Time, endpointLiteral bool) error {
	if local.HostingRoot == "" || !filepath.IsAbs(local.HostingRoot) || filepath.Clean(local.HostingRoot) != local.HostingRoot || local.AdmissionRoot == "" ||
		!filepath.IsAbs(local.AdmissionRoot) || filepath.Clean(local.AdmissionRoot) != local.AdmissionRoot ||
		local.Certificate.PrivateKey == nil || local.ConnectionLimit == 0 || local.ConnectionLimit > 16 ||
		local.DrainTimeout <= 0 || local.DrainTimeout > time.Minute || !endpointLiteral ||
		routecarrier.CarrierProfile(snapshot.CarrierProfile) != routecarrier.ClosedCarrierTCP && routecarrier.CarrierProfile(snapshot.CarrierProfile) != routecarrier.ClosedCarrierQUIC {
		return errors.New("closed JOIN local reservation is incomplete")
	}
	if _, ok := source.Receiver(snapshot, ardp.PurposeDataJoin, now); !ok {
		return errors.New("closed JOIN State projection is unavailable")
	}
	return nil
}

func Start(config Config) (*Handle, error) {
	if config.Authority.CurrentRoute == nil || config.CurrentDuty == nil || config.OpenHost == nil || config.Now == nil {
		return nil, errors.New("closed JOIN dependencies are incomplete")
	}
	local, snapshot := config.Profile, config.Snapshot
	receiver, available := config.Authority.Receiver(snapshot, ardp.PurposeDataJoin, config.Now())
	if !available {
		return nil, errors.New("closed JOIN State changed before reservation")
	}
	spends, err := spending.Open(local.AdmissionRoot, spending.Binding{NetworkID: receiver.NetworkID,
		ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		return nil, err
	}
	limits, err := route.NewClosedDutyLimits(config.Now)
	if err != nil {
		return nil, errors.Join(err, spends.Close())
	}
	host, err := config.OpenHost(local.HostingRoot)
	if err != nil {
		return nil, errors.Join(err, spends.Close())
	}
	pairs, err := route.NewReplenishableClosedJoinPairs(receiver, limits, host.Replenisher(receiver, spends))
	if err != nil {
		return nil, errors.Join(err, spends.Close(), host.Close())
	}
	shared, err := routecarrier.ListenClosedSharedCarrier(routecarrier.CarrierProfile(snapshot.CarrierProfile), config.ListenAddress, local.Certificate,
		func(key [32]byte) bool {
			updated, err := config.CurrentDuty()
			return err == nil && config.Authority.PeerCurrent(updated, key, config.Now())
		}, local.ConnectionLimit)
	if err != nil {
		return nil, errors.Join(err, spends.Close(), host.Close())
	}
	ctx, cancel := context.WithCancel(context.Background())
	running := &closedDataJoinServer{host: host, config: config, receiver: receiver, certificate: local.Certificate, listener: shared,
		pairs: pairs, spends: spends, limits: limits, capacity: make(chan struct{}, local.ConnectionLimit), cancel: cancel,
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

type closedDataJoinServer struct {
	host        Host
	pairs       *route.ClosedJoinPairs
	config      Config
	receiver    route.ClosedRoleReceiver
	certificate tls.Certificate
	listener    routecarrier.ClosedSharedCarrierListener
	spends      *spending.Ledger
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
	monitor := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		monitor <- server.monitorHost(ctx, ticker.C)
	}()
	err := server.accept(ctx)
	stopErr := server.stop()
	server.done <- err
	server.workers.Wait()
	server.pairs.Close()
	// No timeout releases roots while a child still owns a commit or reply.
	server.drainErr = errors.Join(stopErr, server.cleanupErr, server.spends.Close(), <-monitor, server.host.Close())
	close(server.drained)
}

func (server *closedDataJoinServer) monitorHost(ctx context.Context, ticks <-chan time.Time) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticks:
			sample, err := server.host.Sample(ctx, time.Second)
			// Stop cancels the sampling context before joining this monitor.
			// An interrupted observation is not a new Hosting drain cause.
			if ctx.Err() != nil {
				return nil
			}
			if err != nil || sample.Observation.Drain {
				_ = server.stop()
				return errors.Join(err, errors.New("JOIN host allowance requires drain"))
			}
		}
	}
}

func (server *closedDataJoinServer) accept(ctx context.Context) error {
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

func (server *closedDataJoinServer) closeCarrier(connection net.Conn) {
	server.recordAcceptedClose(connection.Close())
}

func (server *closedDataJoinServer) recordAcceptedClose(err error) {
	if err == nil || errors.Is(err, net.ErrClosed) {
		return
	}
	server.cleanupMu.Lock()
	server.cleanupErr = errors.Join(server.cleanupErr, err)
	server.cleanupMu.Unlock()
}
