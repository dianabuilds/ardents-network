package issuer

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
)

const closedIssuerDirectAdjacency = byte(1)

// ClosedTokenListenerConfig contains only one initialized issuer, exact role
// endpoint/carrier and finite local connection bound. State-owning code must
// derive all of them; no Target, holder, permission or authority enters here.
type ClosedTokenListenerConfig struct {
	Issuer          *ClosedTokenIssuer
	SharedListener  routecarrier.ClosedSharedCarrierListener
	NodeHandler     ClosedNodeBootstrapHandler
	CarrierProfile  routecarrier.CarrierProfile
	Endpoint        string
	Certificate     tls.Certificate
	ConnectionLimit uint16
	Clock           func() time.Time
}

// ClosedNodeBootstrapHandler owns the Node-authenticated outer state and may
// invoke serve only after it has completed inner TLS and verified inner HELLO.
type ClosedNodeBootstrapHandler func(context.Context, routecarrier.ClosedSharedCarrier, func(context.Context, io.ReadWriter, [32]byte, ardp.Hello) error)

// ClosedTokenListener owns bounded direct role bootstrap serving. Its caller
// separately owns State refresh and issuer-root lifetime.
type ClosedTokenListener struct {
	issuer     *ClosedTokenIssuer
	listener   routecarrier.ClosedRoleCarrierListener
	shared     routecarrier.ClosedSharedCarrierListener
	node       ClosedNodeBootstrapHandler
	controller *route.ClosedBootstrapController
	clock      func() time.Time
	limit      chan struct{}
	stopOnce   sync.Once
	stopErr    error
	cancel     context.CancelFunc
	drained    chan struct{}
	done       chan error
	stopped    chan struct{}
	workers    sync.WaitGroup
	closeMu    sync.Mutex
	closeErr   error
}

// StartClosedTokenListener starts one selected direct role carrier. All
// unauthenticated direct peers share one adjacency reservation: fresh source
// sockets cannot manufacture additional bootstrap capacity.
func StartClosedTokenListener(ctx context.Context, config ClosedTokenListenerConfig) (*ClosedTokenListener, error) {
	if ctx == nil || config.Issuer == nil || config.ConnectionLimit == 0 || config.ConnectionLimit > 16 || config.Clock == nil || config.Clock().IsZero() ||
		(config.SharedListener == nil && config.NodeHandler != nil) || (config.SharedListener != nil && config.NodeHandler == nil) {
		return nil, errors.New("closed token listener configuration is invalid")
	}
	var listener routecarrier.ClosedRoleCarrierListener
	if config.SharedListener == nil {
		var err error
		listener, err = routecarrier.ListenClosedRoleCarrier(config.CarrierProfile, config.Endpoint, config.Certificate)
		if err != nil {
			return nil, err
		}
	}
	controller, err := route.NewClosedBootstrapController(config.Clock)
	if err != nil {
		if listener != nil {
			_ = listener.Close()
		}
		if config.SharedListener != nil {
			_ = config.SharedListener.Close()
		}
		return nil, err
	}
	owned, cancel := context.WithCancel(ctx)
	running := &ClosedTokenListener{
		issuer: config.Issuer, listener: listener, shared: config.SharedListener, node: config.NodeHandler, controller: controller, clock: config.Clock,
		limit: make(chan struct{}, config.ConnectionLimit), done: make(chan error, 1), stopped: make(chan struct{}),
		cancel: cancel, drained: make(chan struct{}),
	}
	running.workers.Add(2) // Accept handoff and cancellation watcher both join.
	go func() {
		defer running.workers.Done()
		<-owned.Done()
		_ = running.Stop() // A blocked TCP Accept does not observe context alone.
	}()
	go running.serve(owned)
	go func() { running.workers.Wait(); close(running.drained) }()
	return running, nil
}

// Done reports the terminal accept-loop result exactly once.
func (listener *ClosedTokenListener) Done() <-chan error {
	if listener == nil {
		return nil
	}
	return listener.done
}

// Active reports occupied connection-limit slots for Node usage sampling.
func (listener *ClosedTokenListener) Active() uint32 {
	if listener == nil {
		return 0
	}
	return uint32(len(listener.limit))
}

// Stop refuses new peers. Drain joins currently bounded exchanges but does not
// close the issuer root; only its State-owned lifecycle may do that.
func (listener *ClosedTokenListener) Stop() error {
	if listener == nil {
		return nil
	}

	listener.stopOnce.Do(func() {
		close(listener.stopped)
		listener.cancel()
		if listener.listener != nil {
			listener.stopErr = listener.listener.Close()
		} else {
			listener.stopErr = listener.shared.Close()
		}
	})
	return listener.stopErr
}
func (listener *ClosedTokenListener) Drain(ctx context.Context) error {
	if listener == nil || ctx == nil {
		return errors.New("closed token listener drain is invalid")
	}
	stopErr := listener.Stop()
	select {
	case <-listener.drained:
		return errors.Join(stopErr, listener.closeErr)
	case <-ctx.Done():
		return errors.Join(stopErr, ctx.Err())
	}
}

func (listener *ClosedTokenListener) serve(ctx context.Context) {
	defer listener.workers.Done()
	defer listener.Stop()
	var terminal error
	defer func() { listener.done <- terminal }()
	for {
		deadline := listener.clock().UTC().Add(10 * time.Second)
		if listener.shared != nil {
			accepted, err := listener.shared.Accept(ctx, 10*time.Second)
			if err != nil {
				select {
				case <-listener.stopped:
					return
				case <-ctx.Done():
					return
				default:
					if routecarrier.IsClosedSharedPeerFailure(err) {
						continue
					}
					terminal = err
					return
				}
			}
			if ctx.Err() != nil {
				if accepted.Connection != nil {
					listener.closeCarrier(accepted.Connection)
				}
				return
			}
			if accepted.Kind == routecarrier.ClosedSharedNode && accepted.Connection != nil {
				select {
				case listener.limit <- struct{}{}:
					listener.workers.Add(1)
					go listener.serveNode(ctx, accepted)
				default:
					listener.closeCarrier(accepted.Connection)
				}
				continue
			}
			if accepted.Kind != routecarrier.ClosedSharedDirect || accepted.Connection == nil {
				if accepted.Connection != nil {
					listener.closeCarrier(accepted.Connection)
				}
				continue
			}
			listener.startDirect(ctx, accepted.Connection)
			continue
		}
		connection, err := listener.listener.Accept(ctx, deadline)
		if err != nil {
			select {
			case <-listener.stopped:
				return
			case <-ctx.Done():
				return
			default:
				terminal = err
				return
			}
		}
		if ctx.Err() != nil {
			listener.closeCarrier(connection)
			return
		}
		listener.startDirect(ctx, connection)
	}
}

func (listener *ClosedTokenListener) startDirect(ctx context.Context, connection net.Conn) {
	select {
	case listener.limit <- struct{}{}:
		listener.workers.Add(1)
		go listener.serveConnection(ctx, connection)
	default:
		listener.closeCarrier(connection)
	}
}

func (listener *ClosedTokenListener) serveConnection(ctx context.Context, connection net.Conn) {
	defer listener.workers.Done()
	defer func() { <-listener.limit; listener.closeCarrier(connection) }()
	deadline := listener.clock().UTC().Add(10 * time.Second)
	if err := connection.SetDeadline(deadline); err != nil {
		return
	}
	var adjacency [32]byte
	adjacency[0] = closedIssuerDirectAdjacency
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); listener.closeCarrier(connection) })
	defer func() {
		if !stop() {
			<-interrupted
		}
	}()
	if err := listener.issuer.ServeBootstrap(ctx, connection, listener.controller, adjacency); err == nil {
		// A QUIC Write only queues bytes. Closing the whole connection here can
		// discard the successful result before its peer reads it. Retain this
		// bounded direct exchange until the peer closes after consuming its
		// response, or the existing exchange deadline/cancellation interrupts it.
		// Any extra byte also ends this one-operation connection without effects.
		var trailing [1]byte
		_, _ = connection.Read(trailing[:])
	}
}

func (listener *ClosedTokenListener) serveVerified(ctx context.Context, carrier io.ReadWriter, adjacency [32]byte, hello ardp.Hello) error {
	return listener.issuer.ServeBootstrapAfterHello(ctx, carrier, listener.controller, adjacency, hello)
}

func (listener *ClosedTokenListener) serveNode(ctx context.Context, carrier routecarrier.ClosedSharedCarrier) {
	defer listener.workers.Done()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(interrupted)
		_ = carrier.Connection.SetDeadline(time.Now())
		listener.closeCarrier(carrier.Connection)
	})
	defer func() {
		listener.closeCarrier(carrier.Connection)
		if !stop() {
			<-interrupted
		}
		<-listener.limit
	}()
	listener.node(ctx, carrier, listener.serveVerified)
}
