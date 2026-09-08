package credential

import (
	"context"
	"crypto/tls"
	"errors"
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
	CarrierProfile  route.CarrierProfile
	Endpoint        string
	Certificate     tls.Certificate
	ConnectionLimit uint16
	Clock           func() time.Time
}

// ClosedTokenListener owns bounded direct role bootstrap serving. Its caller
// separately owns State refresh and issuer-root lifetime.
type ClosedTokenListener struct {
	issuer     *ClosedTokenIssuer
	listener   route.ClosedRoleCarrierListener
	controller *route.ClosedBootstrapController
	clock      func() time.Time
	limit      chan struct{}
	active     atomic.Uint32
	stopOnce   sync.Once
	done       chan error
	stopped    chan struct{}
	workers    sync.WaitGroup
}

// StartClosedTokenListener starts one selected direct role carrier. All
// unauthenticated direct peers share one adjacency reservation: fresh source
// sockets cannot manufacture additional bootstrap capacity.
func StartClosedTokenListener(ctx context.Context, config ClosedTokenListenerConfig) (*ClosedTokenListener, error) {
	if ctx == nil || config.Issuer == nil || config.ConnectionLimit == 0 || config.ConnectionLimit > 16 || config.Clock == nil || config.Clock().IsZero() {
		return nil, errors.New("closed token listener configuration is invalid")
	}
	listener, err := route.ListenClosedRoleCarrier(config.CarrierProfile, config.Endpoint, config.Certificate)
	if err != nil {
		return nil, err
	}
	controller, err := route.NewClosedBootstrapController(config.Clock)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	running := &ClosedTokenListener{
		issuer: config.Issuer, listener: listener, controller: controller, clock: config.Clock,
		limit: make(chan struct{}, config.ConnectionLimit), done: make(chan error, 1), stopped: make(chan struct{}),
	}
	go running.serve(ctx)
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
	var result error
	listener.stopOnce.Do(func() {
		close(listener.stopped)
		result = listener.listener.Close()
	})
	return result
}
func (listener *ClosedTokenListener) Drain(ctx context.Context) error {
	if listener == nil || ctx == nil {
		return errors.New("closed token listener drain is invalid")
	}
	stopErr := listener.Stop()
	done := make(chan struct{})
	go func() {
		listener.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
		return stopErr
	case <-ctx.Done():
		return errors.Join(stopErr, ctx.Err())
	}
}

func (listener *ClosedTokenListener) serve(ctx context.Context) {
	var terminal error
	defer func() { listener.done <- terminal }()
	for {
		deadline := listener.clock().UTC().Add(10 * time.Second)
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
		select {
		case listener.limit <- struct{}{}:
			listener.active.Add(1)
			listener.workers.Add(1)
			go listener.serveConnection(ctx, connection)
		default:
			_ = connection.Close()
		}
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
	_ = listener.issuer.ServeBootstrap(ctx, connection, listener.controller, adjacency)
}
