package node

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// startClosedForwarding materializes one State-selected adjacent/interior
// receiver. It owns its spend ledger and finite pool until duty withdrawal.
func startClosedForwarding(config runtimeConfig, snapshot dutyFacts) (*probeServer, error) {
	local := config.ClosedForwarding
	if err := validateClosedForwardingProfile(local, config, snapshot, config.now()); err != nil {
		return nil, err
	}
	receiver, available := closedRouteReceiver(config, snapshot, route.ClosedPurposeForwarding, config.now())
	if !available {
		return nil, errors.New("closed forwarding receiver is unavailable")
	}
	spends, err := route.OpenClosedSpendLedger(local.Root, route.ClosedSpendBinding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest,
		ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		return nil, err
	}
	limits, err := route.NewClosedDutyLimits(config.now)
	if err != nil {
		return nil, errors.Join(err, spends.Close())
	}
	bootstrap, err := route.NewClosedBootstrapController(config.now)
	if err != nil {
		return nil, errors.Join(err, spends.Close())
	}
	pool, err := route.NewClosedCarrierPool(config.now)
	if err != nil {
		return nil, errors.Join(err, spends.Close())
	}
	shared, err := route.ListenClosedSharedCarrier(route.CarrierProfile(snapshot.CarrierProfile), snapshot.ProbeEndpoint, local.Certificate,
		func(key [32]byte) bool {
			updated, readErr := currentFacts(config)
			return readErr == nil && closedSharedPeerCurrent(config, updated, key, config.now())
		}, local.ConnectionLimit)
	if err != nil {
		return nil, errors.Join(err, pool.Close(), spends.Close())
	}
	running := newClosedForwardingServer(config, snapshot, local.Certificate, shared, spends, limits, pool, bootstrap, local.ConnectionLimit)
	return &probeServer{Done: running.Done(), Protect: func(bool) {}, Usage: func() (uint64, uint64, uint64) {
		return uint64(running.Active()), uint64(running.Active()), 0
	}, Stop: func() { _ = running.Stop() }, Drain: func(ctx context.Context) error {
		drain, cancel := context.WithTimeout(ctx, local.DrainTimeout)
		defer cancel()
		return running.Drain(drain)
	}}, nil
}

func validateClosedForwardingProfile(local ClosedForwardingProfile, config runtimeConfig, snapshot dutyFacts, now time.Time) error {
	if local.Root == "" || !filepath.IsAbs(local.Root) || filepath.Clean(local.Root) != local.Root || local.Certificate.PrivateKey == nil ||
		local.ConnectionLimit == 0 || local.ConnectionLimit > 16 || local.DrainTimeout <= 0 || local.DrainTimeout > time.Minute ||
		!literalNodeEndpoint(snapshot.ProbeEndpoint) || (route.CarrierProfile(snapshot.CarrierProfile) != route.ClosedCarrierTCP && route.CarrierProfile(snapshot.CarrierProfile) != route.ClosedCarrierQUIC) {
		return errors.New("closed forwarding local profile is incomplete")
	}
	if _, available := closedRouteReceiver(config, snapshot, route.ClosedPurposeForwarding, now); !available {
		return errors.New("closed forwarding State profile is unavailable")
	}
	return nil
}

type closedForwardingServer struct {
	config      runtimeConfig
	snapshot    dutyFacts
	certificate tls.Certificate
	listener    route.ClosedSharedCarrierListener
	spends      *route.ClosedSpendLedger
	limits      *route.ClosedDutyLimits
	pool        *route.ClosedCarrierPool
	bootstrap   *route.ClosedBootstrapController
	sessions    *closedForwardingSessions
	clock       func() time.Time
	limit       chan struct{}
	active      atomic.Uint32
	stopOnce    sync.Once
	stopErr     error
	cancel      context.CancelFunc
	drained     chan struct{}
	done        chan error
	stopped     chan struct{}
	workers     sync.WaitGroup
	outgoingErr error
	drainErr    error
	reapMu      sync.Mutex
	reapErr     error
}

func newClosedForwardingServer(config runtimeConfig, snapshot dutyFacts, certificate tls.Certificate, listener route.ClosedSharedCarrierListener, spends *route.ClosedSpendLedger, limits *route.ClosedDutyLimits, pool *route.ClosedCarrierPool, bootstrap *route.ClosedBootstrapController, limit uint16) *closedForwardingServer {
	ctx, cancel := context.WithCancel(context.Background())
	running := &closedForwardingServer{config: config, snapshot: snapshot, certificate: certificate, listener: listener, spends: spends, limits: limits, pool: pool, bootstrap: bootstrap,
		cancel: cancel, drained: make(chan struct{}),
		clock: config.now, limit: make(chan struct{}, limit), done: make(chan error, 1), stopped: make(chan struct{})}
	running.sessions = newClosedForwardingSessions(&running.workers)
	running.workers.Add(3)
	go running.reap()
	go running.serve(ctx)
	go running.closeOutgoing(ctx)
	go running.finishShutdown()
	return running
}

func (server *closedForwardingServer) Done() <-chan error { return server.done }
func (server *closedForwardingServer) Active() uint32     { return server.active.Load() }

func (server *closedForwardingServer) Stop() error {
	if server == nil {
		return nil
	}
	server.stopOnce.Do(func() {
		close(server.stopped)
		server.cancel()
		server.stopErr = server.listener.Close()
	})
	return server.stopErr
}

func (server *closedForwardingServer) Drain(ctx context.Context) error {
	if server == nil || ctx == nil {
		return errors.New("closed forwarding drain is invalid")
	}
	stopErr := server.Stop()
	select {
	case <-server.drained:
		return errors.Join(stopErr, server.drainErr)
	case <-ctx.Done():
		return errors.Join(stopErr, ctx.Err())
	}
}

func (server *closedForwardingServer) serve(ctx context.Context) {
	defer server.workers.Done()
	defer server.Stop()
	var terminal error
	defer func() { server.done <- terminal }()
	for {
		if err := server.pool.Reap(); err != nil {
			terminal = err
			return
		}
		accepted, err := server.listener.Accept(ctx, 10*time.Second)
		if err != nil {
			select {
			case <-server.stopped:
				server.reapMu.Lock()
				reapErr := server.reapErr
				server.reapMu.Unlock()
				if reapErr != nil {
					terminal = reapErr
				}
				return
			default:
				terminal = err
				return
			}
		}
		select {
		case server.limit <- struct{}{}:
			server.active.Add(1)
			server.workers.Add(1)
			go server.serveAccepted(ctx, accepted)
		default:
			_ = accepted.Connection.Close()
		}
	}
}

func (server *closedForwardingServer) reap() {
	defer server.workers.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-server.stopped:
			return
		case <-ticker.C:
			if err := server.pool.Reap(); err != nil {
				server.reapMu.Lock()
				server.reapErr = err
				server.reapMu.Unlock()
				_ = server.Stop()
				return
			}
		}
	}
}

func (server *closedForwardingServer) serveAccepted(ctx context.Context, accepted route.ClosedSharedCarrier) {
	defer server.workers.Done()
	if accepted.Connection == nil {
		<-server.limit
		server.active.Add(^uint32(0))
		return
	}
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(interrupted)
		_ = accepted.Connection.SetDeadline(time.Now())
		_ = accepted.Connection.Close()
	})
	defer func() {
		_ = accepted.Connection.SetDeadline(time.Now())
		_ = accepted.Connection.Close()
		if !stop() {
			<-interrupted
		}
		<-server.limit
		server.active.Add(^uint32(0))
	}()
	if accepted.Kind == route.ClosedSharedDirect {
		server.serveDirect(ctx, accepted.Connection, nil, [32]byte{}, route.ClosedChildOrdinary, nil)
		return
	}
	if accepted.Kind == route.ClosedSharedNode {
		server.serveOuter(ctx, accepted)
	}
}

func (server *closedForwardingServer) serveOuter(ctx context.Context, carrier route.ClosedSharedCarrier) {
	updated, err := currentFacts(server.config)
	if err != nil {
		return
	}
	receiver, available := closedRouteReceiver(server.config, updated, route.ClosedPurposeForwarding, server.clock())
	if !available {
		return
	}
	deadline := receiver.NotAfter
	outer, err := route.NewClosedOuterHandshake(route.ClosedOuterReceiver{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest,
		ProfileDigest: receiver.ProfileDigest, NodeID: receiver.NodeID, RecordDigest: receiver.RecordDigest, DutyGeneration: receiver.DutyGeneration,
		RoleDomain: receiver.RoleDomain, Subrole: receiver.Subrole, Deadline: deadline}, server.limits, server.clock)
	if err != nil {
		return
	}
	serveClosedOuter(ctx, carrier.Connection, outer, func(childContext context.Context, lane *route.ClosedOuterBridgeLane) {
		server.serveInner(childContext, lane, deadline, carrier.NodeKey)
	})
}

func (server *closedForwardingServer) serveInner(ctx context.Context, lane *route.ClosedOuterBridgeLane, deadline time.Time, incomingKey [32]byte) {
	status := byte(1)
	defer func() { _ = lane.CloseWithStatus(status) }()
	secured, err := route.AcceptClosedRoleTLS(ctx, lane, server.certificate, deadline)
	if err != nil {
		return
	}
	defer func() {
		if secured.CloseWrite() != nil {
			status = 1
		}
	}()
	if err := lane.BeginInnerHello(); err != nil {
		return
	}
	helloFrame, err := route.ReadClosedLaneFrame(secured)
	if err != nil {
		return
	}
	if helloFrame.Kind != 1 || helloFrame.Lane != 0 {
		return
	}
	hello, err := route.DecodeClosedHello(helloFrame.Body)
	if err != nil || lane.Activate(hello) != nil {
		return
	}
	if server.serveDirect(ctx, secured, &helloFrame, incomingKey, lane.Restriction(), lane) == nil {
		status = 0
	}
}

func (server *closedForwardingServer) serveDirect(ctx context.Context, connection net.Conn, first *route.ClosedLaneFrame, incomingKey [32]byte, restriction route.ClosedChildRestriction, outerLane *route.ClosedOuterBridgeLane) error {

	initialDeadline := server.clock().UTC().Add(10 * time.Second)
	if err := connection.SetDeadline(initialDeadline); err != nil {
		return err
	}
	exporter, err := route.ClosedRoleTLSExporter(connection)
	if err != nil {
		return err
	}
	updated, err := currentFacts(server.config)
	if err != nil {
		return err
	}
	receiver, available := closedRouteReceiver(server.config, updated, route.ClosedPurposeForwarding, server.clock())
	if !available {
		return errors.New("closed forwarding receiver is unavailable")
	}
	admission, err := route.NewClosedAdmissionChannel(receiver, server.spends, server.limits, exporter, closedRoleTokenVerifier(server.config, receiver), server.clock)
	if err != nil {
		return err
	}
	var forwarding *route.ClosedForwardingChannel
	var writer sync.Mutex
	write := func(frame route.ClosedLaneFrame) error {
		writer.Lock()
		defer writer.Unlock()
		if forwarding != nil {
			if err := forwarding.AccountOutput(frame); err != nil {
				return err
			}
		}
		return route.WriteClosedLaneFrame(connection, frame)
	}
	links := make(map[uint32]*closedForwardingLink)
	defer func() {
		_ = connection.SetDeadline(time.Now())
		_ = connection.Close()
		for _, link := range links {
			_ = link.close()
		}
		if forwarding != nil {
			forwarding.Cancel()
		}
	}()
	var hello route.ClosedHello
	helloSize := 0
	accept := func(frame route.ClosedLaneFrame) error {
		if restriction != route.ClosedChildOrdinary && restriction != route.ClosedChildIssuerBootstrap {
			return errors.New("closed forwarding child restriction is invalid")
		}
		if restriction == route.ClosedChildIssuerBootstrap && frame.Kind == 2 {
			return errors.New("closed bootstrap child cannot accept private admission")
		}
		if forwarding == nil {
			if frame.Kind == 3 {
				var deadline time.Time
				var bootstrapErr error
				forwarding, deadline, bootstrapErr = server.admitBootstrap(receiver, incomingKey, hello, helloSize, frame)
				if bootstrapErr != nil {
					return bootstrapErr
				}
				if err := connection.SetDeadline(deadline); err != nil {
					return err
				}
				accepted, err := route.ClosedAcceptFrame(0, 64<<10)
				if err != nil {
					return err
				}
				return write(accepted)
			}
			lease, admitErr := admission.Accept(frame)
			if admitErr != nil {
				return admitErr
			}
			if lease.Class == 0 {
				hello, _ = route.DecodeClosedHello(frame.Body)
				helloSize = 16 + len(frame.Body)
				return nil
			}
			if outerLane != nil {
				if err := outerLane.Admit(&lease, connection); err != nil {
					lease.Release()
					return err
				}
			}
			forwarding, admitErr = route.NewClosedForwardingChannel(&lease, func(open route.ClosedOpen) error {
				updated, readErr := currentFacts(server.config)
				if readErr != nil {
					return readErr
				}
				_, readErr = closedForwardRecipient(server.config, updated, open, server.clock())
				return readErr
			}, server.clock)
			if admitErr != nil {
				lease.Release()
				return admitErr
			}
			if err := connection.SetDeadline(lease.Deadline); err != nil {
				forwarding.Cancel()
				return err
			}
			accepted, frameErr := route.ClosedAcceptFrame(0, 64<<10)
			if frameErr != nil {
				return frameErr
			}
			return write(accepted)
		}
		// Serialize CLOSE with actual output so a previously queued frame cannot
		// pass the retirement boundary while its writer is already admitted.
		if frame.Kind == 9 {
			writer.Lock()
		}
		_, acceptErr := forwarding.Accept(frame)
		if frame.Kind == 9 {
			writer.Unlock()
		}
		if acceptErr != nil {
			return acceptErr
		}
		return server.drainForwarding(ctx, forwarding, links, write, func() { _ = connection.Close() })
	}
	if first != nil {
		if err := accept(*first); err != nil {
			return err
		}
	}
	for {
		frame, readErr := route.ReadClosedLaneFrame(connection)
		if readErr != nil {
			return readErr
		}
		if err := accept(frame); err != nil {
			return err
		}
	}
}

func closedRoleTokenVerifier(config runtimeConfig, receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
	return func(input route.ClosedAdmissionVerification) (time.Time, error) {
		if len(input.Token) != 354 || input.Class < 1 || input.Class > 3 || config.CurrentClosedProfile == nil {
			return time.Time{}, errors.New("closed forwarding token is unavailable")
		}
		profile, available := config.CurrentClosedProfile()
		now := config.now().UTC()
		if !available || profile.NetworkID != receiver.NetworkID || profile.StateGeneration != receiver.StateGeneration || profile.StateDigest != receiver.StateDigest ||
			profile.Digest != receiver.ProfileDigest || profile.NotBefore.After(now) || !now.Before(profile.NotAfter) {
			return time.Time{}, errors.New("closed forwarding token is unavailable")
		}
		var keyID [32]byte
		copy(keyID[:], input.Token[66:98])
		for index := uint8(0); index < profile.TokenKeyCount; index++ {
			key := profile.TokenKeys[index]
			if key.Class != input.Class || key.WindowStart != now.Truncate(time.Hour) || sha256.Sum256(key.SPKI[:]) != keyID {
				continue
			}
			context := credential.ClosedTokenContext{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID,
				IssuerNodeID: profile.IssuerNodeID, ReceiverDutyGeneration: receiver.DutyGeneration, Class: input.Class, WindowStart: key.WindowStart}
			if credential.VerifyClosedToken(context, key.SPKI[:], input.Token) == nil {
				return key.WindowStart, nil
			}
		}
		return time.Time{}, errors.New("closed forwarding token is unavailable")
	}
}
