package credential

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// This controlled listener pauses an already accepted socket at the handoff
// boundary. It isolates ownership/joining, not peer authentication.
type closedIssuerPausedAccept struct {
	connection net.Conn
	kind       routecarrier.ClosedSharedCarrierKind
	entered    chan struct{}
	release    chan struct{}
	once       sync.Once
	closed     chan struct{}
	closeOnce  sync.Once
}

func (listener *closedIssuerPausedAccept) Accept(ctx context.Context, _ time.Duration) (routecarrier.ClosedSharedCarrier, error) {
	first := false
	listener.once.Do(func() { first = true; close(listener.entered) })
	if first {
		<-listener.release
		kind := listener.kind
		if kind == 0 {
			kind = routecarrier.ClosedSharedNode
		}
		return routecarrier.ClosedSharedCarrier{Kind: kind, Connection: listener.connection, NodeKey: [32]byte{1}}, nil
	}
	select {
	case <-ctx.Done():
		return routecarrier.ClosedSharedCarrier{}, ctx.Err()
	case <-listener.closed:
		return routecarrier.ClosedSharedCarrier{}, net.ErrClosed
	}
}
func (listener *closedIssuerPausedAccept) Close() error {
	listener.closeOnce.Do(func() { close(listener.closed) })
	return nil
}

func TestClosedTokenListenerDrainJoinsAcceptedSocketHandoff(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()
	shared := &closedIssuerPausedAccept{connection: server, entered: make(chan struct{}),
		release: make(chan struct{}), closed: make(chan struct{})}
	var handled atomic.Uint32
	listener, err := StartClosedTokenListener(t.Context(), ClosedTokenListenerConfig{Issuer: &ClosedTokenIssuer{}, SharedListener: shared,
		ConnectionLimit: 1, Clock: time.Now, NodeHandler: func(ctx context.Context, carrier routecarrier.ClosedSharedCarrier,
			_ func(context.Context, io.ReadWriter, [32]byte, ardp.Hello) error) {
			handled.Add(1)
			_ = carrier.Connection.Close()
		}})
	if err != nil {
		t.Fatal(err)
	}
	<-shared.entered
	bounded, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	err = listener.Drain(bounded)
	cancel()
	if err == nil {
		t.Error("Drain completed while an accepted socket was still being handed off")
	}
	if tokenListenerJoined(listener) {
		t.Error("listener reported a joined handoff before the accepted socket returned")
	}
	close(shared.release)
	joined, finish := context.WithTimeout(t.Context(), time.Second)
	defer finish()
	if err := listener.Drain(joined); err != nil {
		t.Fatal(err)
	}
	if !tokenListenerJoined(listener) {
		t.Fatal("listener did not report a completed join")
	}
	if handled.Load() != 0 {
		t.Fatal("stopped listener launched the delayed Node handler")
	}
}

func TestClosedTokenListenerStopCancelsIdleNode(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()
	shared := &closedIssuerPausedAccept{connection: server, entered: make(chan struct{}),
		release: make(chan struct{}), closed: make(chan struct{})}
	close(shared.release)
	started, stopped := make(chan struct{}), make(chan struct{})
	parent, cancel := context.WithCancel(t.Context())
	defer cancel()
	listener, err := StartClosedTokenListener(parent, ClosedTokenListenerConfig{Issuer: &ClosedTokenIssuer{}, SharedListener: shared,
		ConnectionLimit: 1, Clock: time.Now, NodeHandler: func(ctx context.Context, carrier routecarrier.ClosedSharedCarrier,
			_ func(context.Context, io.ReadWriter, [32]byte, ardp.Hello) error) {
			defer close(stopped)
			defer carrier.Connection.Close()
			close(started)
			<-ctx.Done()
		}})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	drain, finish := context.WithTimeout(t.Context(), time.Second)
	err = listener.Drain(drain)
	finish()
	if err != nil {
		t.Error(err)
	}
	// Unblock the old implementation on test failure, without hiding it.
	cancel()
	<-stopped
}

func TestClosedTokenListenerParentCancellationClosesIdleTCPAccept(t *testing.T) {
	certificate, _ := closedTokenListenerCertificate(t)
	parent, cancel := context.WithCancel(t.Context())
	defer cancel()
	listener, err := StartClosedTokenListener(parent, ClosedTokenListenerConfig{Issuer: &ClosedTokenIssuer{},
		CarrierProfile: routecarrier.ClosedCarrierTCP, Endpoint: closedTokenListenerEndpoint(t, routecarrier.ClosedCarrierTCP),
		Certificate: certificate, ConnectionLimit: 1, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Stop()
	cancel()
	select {
	case err := <-listener.Done():
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("parent cancellation did not close idle TCP Accept")
	}
	drain, finish := context.WithTimeout(t.Context(), time.Second)
	defer finish()
	if err := listener.Drain(drain); err != nil {
		t.Fatal(err)
	}
}

// tokenListenerJoined reports worker join through the package-internal
// drained channel after the exported Joined projection was retired (F-17):
// Node owns the late root close and reads the final Drain result instead.
func tokenListenerJoined(listener *ClosedTokenListener) bool {
	select {
	case <-listener.drained:
		return true
	default:
		return false
	}
}
