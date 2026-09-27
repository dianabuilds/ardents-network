package introduction

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
)

// Profile reserves the local durable spend root. Current State supplies the
// receiving identity, role and Carrier.
type Profile struct {
	AdmissionRoot   string
	Certificate     tls.Certificate
	ConnectionLimit uint16
	DrainTimeout    time.Duration
}

// Config contains the selected duty and only the callbacks used by this role.
// VerifyAdmission retains the process Hosting reservation for the lease.
type Config struct {
	Profile         Profile
	Snapshot        state.NodeDuty
	Authority       authority.Source
	CurrentDuty     func() (state.NodeDuty, error)
	VerifyAdmission func(route.ClosedRoleReceiver) route.ClosedAdmissionVerifier
	Now             func() time.Time
	ListenAddress   string
}

func Validate(local Profile, source authority.Source, snapshot state.NodeDuty, now time.Time, endpointLiteral bool) error {
	if local.AdmissionRoot == "" ||
		!filepath.IsAbs(local.AdmissionRoot) || filepath.Clean(local.AdmissionRoot) != local.AdmissionRoot ||
		local.Certificate.PrivateKey == nil || local.ConnectionLimit == 0 || local.ConnectionLimit > 16 ||
		local.DrainTimeout <= 0 || local.DrainTimeout > time.Minute || !endpointLiteral ||
		routecarrier.CarrierProfile(snapshot.CarrierProfile) != routecarrier.ClosedCarrierTCP && routecarrier.CarrierProfile(snapshot.CarrierProfile) != routecarrier.ClosedCarrierQUIC {
		return errors.New("closed Introduction local reservation is incomplete")
	}
	if _, ok := source.Receiver(snapshot, ardp.PurposeIntroduction, now); !ok {
		return errors.New("closed Introduction State projection is unavailable")
	}
	return nil
}

// Start transfers all accepted children and durable roots to one server.
func Start(config Config) (*Server, error) {
	local, snapshot := config.Profile, config.Snapshot
	receiver, available := config.Authority.Receiver(snapshot, ardp.PurposeIntroduction, config.Now())
	if !available {
		return nil, errors.New("closed Introduction State changed before reservation")
	}
	spends, err := replay.Open(local.AdmissionRoot, replay.Binding{NetworkID: receiver.NetworkID,
		ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		return nil, err
	}
	slotFloor, err := spends.IntroductionSlots()
	if err != nil {
		return nil, errors.Join(err, spends.Close())
	}
	limits, err := route.NewClosedDutyLimits(config.Now)
	if err != nil {
		return nil, errors.Join(err, spends.Close())
	}
	shared, err := routecarrier.ListenClosedSharedCarrier(routecarrier.CarrierProfile(snapshot.CarrierProfile), config.ListenAddress, local.Certificate,
		func(key [32]byte) bool {
			updated, err := config.CurrentDuty()
			return err == nil && config.Authority.PeerCurrent(updated, key, config.Now())
		}, local.ConnectionLimit)
	if err != nil {
		return nil, errors.Join(err, spends.Close())
	}
	ctx, cancel := context.WithCancel(context.Background())
	running := &Server{config: config, receiver: receiver, certificate: local.Certificate, listener: shared,
		slots: make(map[[32]byte]*closedIntroductionSlot), slotFloor: slotFloor, spends: spends, limits: limits, capacity: make(chan struct{}, local.ConnectionLimit), cancel: cancel,
		done: make(chan error, 1), drained: make(chan struct{})}
	go running.run(ctx)
	return running, nil
}

// Server owns one selected Introduction role through joined shutdown.
type Server struct {
	slotFloor   *replay.IntroductionSlots
	config      Config
	receiver    route.ClosedRoleReceiver
	certificate tls.Certificate
	listener    routecarrier.ClosedSharedCarrierListener
	slotsMu     sync.Mutex
	slots       map[[32]byte]*closedIntroductionSlot
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

func (server *Server) stop() error {
	server.stopOnce.Do(func() {
		server.cancel()
		server.stopErr = server.listener.Close()
	})
	return server.stopErr
}

func (server *Server) run(ctx context.Context) {
	err := server.accept(ctx)
	stopErr := server.stop()
	server.done <- err
	server.workers.Wait()
	// No timeout releases roots while a child still owns a commit or reply.
	server.drainErr = errors.Join(stopErr, server.spends.Close(), server.cleanupErr)
	close(server.drained)
}

func (server *Server) accept(ctx context.Context) error {
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

func (server *Server) closeCarrier(connection net.Conn) {
	err := connection.Close()
	if errors.Is(err, net.ErrClosed) {
		return
	}
	server.recordCleanup(err)
}

func (server *Server) recordCleanup(err error) {
	if err == nil {
		return
	}
	server.cleanupMu.Lock()
	server.cleanupErr = errors.Join(server.cleanupErr, err)
	server.cleanupMu.Unlock()
}

// Done reports the listener's terminal cause; Joined closes after every child
// and the spend root have completed cleanup.
func (server *Server) Done() <-chan error      { return server.done }
func (server *Server) Joined() <-chan struct{} { return server.drained }
func (server *Server) Stop() error             { return server.stop() }
func (server *Server) Usage() (uint64, uint64, uint64) {
	active := uint64(server.active.Load())
	return active, active, 0
}
func (server *Server) Drain(ctx context.Context, timeout time.Duration) error {
	_ = server.stop()
	bounded, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	select {
	case <-server.drained:
		return server.drainErr
	case <-bounded.Done():
		return bounded.Err()
	}
}
