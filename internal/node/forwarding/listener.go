package forwarding

import (
	"context"
	"crypto/tls"
	"errors"
	nodeouter "github.com/dianabuilds/ardents-network/internal/node/outer"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type forwardServer struct {
	dependencies     dependencies
	certificate      tls.Certificate
	listener         routecarrier.ClosedSharedCarrierListener
	receiving        *receivingResources
	pool             *routecarrier.ClosedCarrierPool
	host             Host
	sessions         *sessionSet
	clock            func() time.Time
	limit            chan struct{}
	active           atomic.Uint32
	stopOnce         sync.Once
	stopErr          error
	cancel           context.CancelFunc
	drained          chan struct{}
	done             chan error
	stopped          chan struct{}
	workers          sync.WaitGroup
	outgoingErr      error
	acceptedCloseMu  sync.Mutex
	acceptedCloseErr error
	drainErr         error
	reapMu           sync.Mutex
	reapErr          error
}

func newServerWithHost(dependencies dependencies, certificate tls.Certificate, listener routecarrier.ClosedSharedCarrierListener, receiving *receivingResources, pool *routecarrier.ClosedCarrierPool, host Host, limit uint16, clock func() time.Time) *forwardServer {
	ctx, cancel := context.WithCancel(context.Background())
	running := &forwardServer{dependencies: dependencies, certificate: certificate, listener: listener, receiving: receiving, pool: pool,
		host: host, cancel: cancel, drained: make(chan struct{}),
		clock: clock, limit: make(chan struct{}, limit), done: make(chan error, 1), stopped: make(chan struct{})}
	running.sessions = newSessionSet()
	running.workers.Add(3)
	go running.reap()
	go running.serve(ctx)
	go running.closeOutgoing(ctx)
	go running.finishShutdown()
	return running
}

func (server *forwardServer) Done() <-chan error { return server.done }
func (server *forwardServer) Active() uint32     { return server.active.Load() }

func (server *forwardServer) Stop() error {
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

func (server *forwardServer) Drain(ctx context.Context) error {
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

func (server *forwardServer) serve(ctx context.Context) {
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
				if routecarrier.IsClosedSharedPeerFailure(err) {
					continue
				}
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
			server.closeAcceptedCarrier(accepted.Connection)
		}
	}
}

func (server *forwardServer) reap() {
	defer server.workers.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-server.stopped:
			return
		case <-ticker.C:
			if server.host != nil {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				sample, err := server.host.Sample(ctx, time.Second)
				cancel()
				if err != nil || sample.Observation.Drain {
					if err == nil {
						err = errors.New("closed forwarding host allowance requires drain")
					}
					server.reapMu.Lock()
					server.reapErr = errors.Join(server.reapErr, err)
					server.reapMu.Unlock()
					_ = server.Stop()
					return
				}
			}
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

func (server *forwardServer) serveAccepted(ctx context.Context, accepted routecarrier.ClosedSharedCarrier) {
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
		server.closeAcceptedCarrier(accepted.Connection)
	})
	defer func() {
		_ = accepted.Connection.SetDeadline(time.Now())
		server.closeAcceptedCarrier(accepted.Connection)
		if !stop() {
			<-interrupted
		}
		<-server.limit
		server.active.Add(^uint32(0))
	}()
	if accepted.Kind == routecarrier.ClosedSharedDirect {
		server.serveDirect(ctx, accepted.Connection, nil, [32]byte{}, route.ClosedChildOrdinary, nil)
		return
	}
	if accepted.Kind == routecarrier.ClosedSharedNode {
		server.serveOuter(ctx, accepted)
	}
}

func (server *forwardServer) serveOuter(ctx context.Context, carrier routecarrier.ClosedSharedCarrier) {
	updated, err := server.dependencies.current()
	if err != nil {
		return
	}
	receiver, available := server.dependencies.authority.Receiver(updated, ardp.PurposeForwarding, server.clock())
	if !available {
		return
	}
	deadline := receiver.NotAfter
	outer, err := route.NewClosedOuterHandshake(route.ClosedOuterReceiver{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest,
		ProfileDigest: receiver.ProfileDigest, NodeID: receiver.NodeID, RecordDigest: receiver.RecordDigest, DutyGeneration: receiver.DutyGeneration,
		RoleDomain: receiver.RoleDomain, Subrole: receiver.Subrole, Deadline: deadline}, server.receiving.limits, server.clock)
	if err != nil {
		return
	}
	server.recordAcceptedClose(nodeouter.Serve(ctx, carrier.Connection, outer, func(childContext context.Context, lane *route.ClosedOuterBridgeLane) {
		server.serveInner(childContext, lane, deadline, carrier.NodeKey)
	}))
}

func (server *forwardServer) serveInner(ctx context.Context, lane *route.ClosedOuterBridgeLane, deadline time.Time, incomingKey [32]byte) {
	status := byte(1)
	defer func() { _ = lane.CloseWithStatus(status) }()
	secured, err := routecarrier.AcceptClosedRoleTLS(ctx, lane, server.certificate, deadline)
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
	helloFrame, err := ardp.ReadFrame(secured)
	if err != nil {
		return
	}
	if helloFrame.Kind != 1 || helloFrame.Lane != 0 {
		return
	}
	hello, err := ardp.DecodeHello(helloFrame.Body)
	if err != nil || lane.Activate(hello) != nil {
		return
	}
	if server.serveDirect(ctx, secured, &helloFrame, incomingKey, lane.Restriction(), lane) == nil {
		status = 0
	}
}

func (server *forwardServer) serveDirect(ctx context.Context, connection net.Conn, first *ardp.Frame, incomingKey [32]byte, restriction route.ClosedChildRestriction, outerLane *route.ClosedOuterBridgeLane) (result error) {
	probeID := diagnostic357ForwardIDs.Add(1)
	probePhase := "setup"
	probeKind := uint8(0)
	defer func() { diagnostic357Forward("parent", probeID, probePhase, probeKind, result) }()

	initialDeadline := server.clock().UTC().Add(10 * time.Second)
	if err := connection.SetDeadline(initialDeadline); err != nil {
		return err
	}
	exporter, err := routecarrier.ClosedRoleTLSExporter(connection)
	if err != nil {
		return err
	}
	updated, err := server.dependencies.current()
	if err != nil {
		return err
	}
	receiver, available := server.dependencies.authority.Receiver(updated, ardp.PurposeForwarding, server.clock())
	if !available {
		return errors.New("closed forwarding receiver is unavailable")
	}
	admission, err := route.NewClosedAdmissionChannel(receiver, server.receiving.spends, server.receiving.limits, exporter,
		server.dependencies.verify(receiver), server.clock)
	if err != nil {
		return err
	}
	var forwarding *route.ClosedForwardingChannel
	var writer sync.Mutex
	write := func(frame ardp.Frame) error {
		writer.Lock()
		defer writer.Unlock()
		if forwarding != nil {
			if err := forwarding.AccountOutput(frame); err != nil {
				return err
			}
		}
		return ardp.WriteFrame(connection, frame)
	}
	links := make(map[uint32]*forwardLink)
	completed := make(chan struct{}, 1)
	wakeParent := func() {
		select {
		case completed <- struct{}{}:
		default:
		}
	}
	openings := newOpenings(ctx, wakeParent)
	type parentRead struct {
		frame ardp.Frame
		err   error
	}
	reads := make(chan parentRead, 1)
	stopReader := make(chan struct{})
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			frame, err := ardp.ReadFrame(connection)
			select {
			case reads <- parentRead{frame: frame, err: err}:
			case <-stopReader:
				return
			}
			if err != nil {
				return
			}
		}
	}()
	defer func() {
		close(stopReader)
		_ = connection.SetDeadline(time.Now())
		_ = connection.Close()
		<-readerDone
		for _, link := range links {
			_ = link.close()
		}
		result = errors.Join(result, openings.close(links))
		if forwarding != nil {
			result = errors.Join(result, forwarding.Cancel())
		}
	}()
	var hello ardp.Hello
	helloSize := 0
	accept := func(frame ardp.Frame) error {
		probePhase = "admission"
		probeKind = uint8(frame.Kind)
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
				accepted, err := ardp.AcceptFrame(0, 64<<10)
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
				hello, _ = ardp.DecodeHello(frame.Body)
				helloSize = 16 + len(frame.Body)
				return nil
			}
			if outerLane != nil {
				if err := outerLane.Admit(&lease, connection); err != nil {
					return errors.Join(err, lease.Release())
				}
			}
			forwarding, admitErr = route.NewReplenishableClosedForwardingChannel(&lease, func(open route.ClosedOpen) error {
				updated, readErr := server.dependencies.current()
				if readErr != nil {
					return readErr
				}
				_, readErr = recipient(server.dependencies.authority, updated, open, server.clock(), server.dependencies.literalEndpoint)
				return readErr
			}, server.dependencies.replenish(receiver, server.receiving.spends), server.clock)
			if admitErr != nil {
				return errors.Join(admitErr, lease.Release())
			}
			if err := connection.SetDeadline(lease.Deadline); err != nil {
				return errors.Join(err, forwarding.Cancel())
			}
			accepted, frameErr := ardp.AcceptFrame(0, 64<<10)
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
		probePhase = "frame-accept"
		_, acceptErr := forwarding.Accept(frame)
		if frame.Kind == 9 {
			writer.Unlock()
		}
		if acceptErr != nil {
			return acceptErr
		}
		if frame.Kind == 2 {
			accepted, frameErr := ardp.AcceptFrame(0, 64<<10)
			if frameErr != nil {
				return frameErr
			}
			if frameErr = write(accepted); frameErr != nil {
				return frameErr
			}
		}
		probePhase = "frame-drain"
		return server.drainForwarding(ctx, forwarding, links, openings, write, func() { _ = connection.Close() })
	}
	if first != nil {
		if err := accept(*first); err != nil {
			return err
		}
	}
	for {
		select {
		case input := <-reads:
			if input.err != nil {
				probePhase = "parent-read"
				return input.err
			}
			if err := accept(input.frame); err != nil {
				return err
			}
		case <-completed:
			probePhase = "completed-drain"
			if err := server.drainForwarding(ctx, forwarding, links, openings, write, func() { _ = connection.Close() }); err != nil {
				return err
			}
		case <-ctx.Done():
			probePhase = "context"
			return ctx.Err()
		}
	}
}

// closeAcceptedCarrier records cleanup failures for the joined duty result.
func (server *forwardServer) closeAcceptedCarrier(connection net.Conn) {
	server.recordAcceptedClose(connection.Close())
}

func (server *forwardServer) recordAcceptedClose(err error) {
	if err == nil || errors.Is(err, net.ErrClosed) {
		return
	}
	server.acceptedCloseMu.Lock()
	server.acceptedCloseErr = errors.Join(server.acceptedCloseErr, err)
	server.acceptedCloseMu.Unlock()
}

// Register this worker before starting the shared Wait. It interrupts outgoing
// reads, writes and HELLOs without taking session locks. Joining inside Carrier
// Close would deadlock: a reader's exact-lease invalidation needs the pool lock
// held while the pool closes its physical transports.
func (server *forwardServer) closeOutgoing(ctx context.Context) {
	defer server.workers.Done()
	<-ctx.Done()
	server.outgoingErr = server.pool.Close()
}

// Join every accepted producer before the session owner waits for its readers.
// A producer can publish a late successful handshake, but no reader can be
// added after joinedResult starts its final Wait. A timeout waiting for drained
// does not release roots: this sole owner retains them until both layers join.
func (server *forwardServer) finishShutdown() {
	server.workers.Wait()
	sessionErr := server.sessions.joinedResult()
	var hostErr error
	if server.host != nil {
		hostErr = server.host.Close()
	}
	server.drainErr = errors.Join(server.outgoingErr, sessionErr, server.receiving.Close(), hostErr, server.acceptedCloseErr)
	close(server.drained)
}
