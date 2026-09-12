package credential

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

const closedIssuerDirectAdjacency = byte(1)

// ClosedTokenListenerConfig contains only one initialized issuer, exact role
// endpoint/carrier and finite local connection bound. State-owning code must
// derive all of them; no Target, holder, permission or authority enters here.
type ClosedTokenListenerConfig struct {
	Issuer          *ClosedTokenIssuer
	SharedListener  route.ClosedSharedCarrierListener
	NodeHandler     ClosedNodeBootstrapHandler
	CarrierProfile  route.CarrierProfile
	Endpoint        string
	Certificate     tls.Certificate
	ConnectionLimit uint16
	Clock           func() time.Time
}

// ClosedNodeBootstrapHandler owns the Node-authenticated outer state and may
// invoke serve only after it has completed inner TLS and verified inner HELLO.
type ClosedNodeBootstrapHandler func(context.Context, route.ClosedSharedCarrier, func(context.Context, io.ReadWriter, [32]byte, route.ClosedHello) error)

// ClosedTokenListener owns bounded direct role bootstrap serving. Its caller
// separately owns State refresh and issuer-root lifetime.
type ClosedTokenListener struct {
	issuer     *ClosedTokenIssuer
	listener   route.ClosedRoleCarrierListener
	shared     route.ClosedSharedCarrierListener
	node       ClosedNodeBootstrapHandler
	controller *route.ClosedBootstrapController
	clock      func() time.Time
	limit      chan struct{}
	active     atomic.Uint32
	stopOnce   sync.Once
	stopErr    error
	cancel     context.CancelFunc
	drained    chan struct{}
	done       chan error
	stopped    chan struct{}
	workers    sync.WaitGroup
}

// StartClosedTokenListener starts one selected direct role carrier. All
// unauthenticated direct peers share one adjacency reservation: fresh source
// sockets cannot manufacture additional bootstrap capacity.
func StartClosedTokenListener(ctx context.Context, config ClosedTokenListenerConfig) (*ClosedTokenListener, error) {
	if ctx == nil || config.Issuer == nil || config.ConnectionLimit == 0 || config.ConnectionLimit > 16 || config.Clock == nil || config.Clock().IsZero() ||
		(config.SharedListener == nil && config.NodeHandler != nil) || (config.SharedListener != nil && config.NodeHandler == nil) {
		return nil, errors.New("closed token listener configuration is invalid")
	}
	var listener route.ClosedRoleCarrierListener
	if config.SharedListener == nil {
		var err error
		listener, err = route.ListenClosedRoleCarrier(config.CarrierProfile, config.Endpoint, config.Certificate)
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

func (listener *ClosedTokenListener) Active() uint32 {
	if listener == nil {
		return 0
	}
	return listener.active.Load()
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
		return stopErr
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
					terminal = err
					return
				}
			}
			if ctx.Err() != nil {
				if accepted.Connection != nil {
					_ = accepted.Connection.Close()
				}
				return
			}
			if accepted.Kind == route.ClosedSharedNode && accepted.Connection != nil {
				select {
				case listener.limit <- struct{}{}:
					listener.active.Add(1)
					listener.workers.Add(1)
					go listener.serveNode(ctx, accepted)
				default:
					_ = accepted.Connection.Close()
				}
				continue
			}
			if accepted.Kind != route.ClosedSharedDirect || accepted.Connection == nil {
				if accepted.Connection != nil {
					_ = accepted.Connection.Close()
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
			_ = connection.Close()
			return
		}
		listener.startDirect(ctx, connection)
	}
}

func (listener *ClosedTokenListener) startDirect(ctx context.Context, connection net.Conn) {
	select {
	case listener.limit <- struct{}{}:
		listener.active.Add(1)
		listener.workers.Add(1)
		go listener.serveConnection(ctx, connection)
	default:
		_ = connection.Close()
	}
}

func (listener *ClosedTokenListener) serveConnection(ctx context.Context, connection net.Conn) {
	defer listener.workers.Done()
	defer func() { <-listener.limit; listener.active.Add(^uint32(0)); _ = connection.Close() }()
	deadline := listener.clock().UTC().Add(10 * time.Second)
	if err := connection.SetDeadline(deadline); err != nil {
		return
	}
	var adjacency [32]byte
	adjacency[0] = closedIssuerDirectAdjacency
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); _ = connection.Close() })
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

func (listener *ClosedTokenListener) serveVerified(ctx context.Context, carrier io.ReadWriter, adjacency [32]byte, hello route.ClosedHello) error {
	return listener.issuer.ServeBootstrapAfterHello(ctx, carrier, listener.controller, adjacency, hello)
}

func (listener *ClosedTokenListener) serveNode(ctx context.Context, carrier route.ClosedSharedCarrier) {
	defer listener.workers.Done()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(interrupted)
		_ = carrier.Connection.SetDeadline(time.Now())
		_ = carrier.Connection.Close()
	})
	defer func() {
		_ = carrier.Connection.Close()
		if !stop() {
			<-interrupted
		}
		<-listener.limit
		listener.active.Add(^uint32(0))
	}()
	listener.node(ctx, carrier, listener.serveVerified)
}
