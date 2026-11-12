package tls

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

type terminalTestListener struct {
	closed  chan struct{}
	once    sync.Once
	accepts atomic.Int32
}

func (l *terminalTestListener) Accept() (net.Conn, error) {
	l.accepts.Add(1)
	<-l.closed
	return nil, net.ErrClosed
}
func (l *terminalTestListener) Close() error   { l.once.Do(func() { close(l.closed) }); return nil }
func (l *terminalTestListener) Addr() net.Addr { return nil }

func terminalListenerFixture() (*sharedListener, *terminalTestListener) {
	native := &terminalTestListener{closed: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	return &sharedListener{listener: native, handshakes: make(chan struct{}, 1), results: make(chan handshakeResult, 1), acceptDone: make(chan struct{}), lifetime: ctx, stop: cancel}, native
}

func TestSharedListenerCanceledWaitDoesNotDispatch(t *testing.T) {
	listener, native := terminalListenerFixture()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := listener.Accept(ctx, time.Second)
	if got.Connection != nil {
		_ = got.Connection.Close()
	}
	if err == nil {
		t.Error("already canceled wait acquired an authentication")
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if native.accepts.Load() != 0 {
		t.Fatal("already canceled wait started native acceptance")
	}
}

func TestSharedListenerClosedOwnerDoesNotDispatch(t *testing.T) {
	listener, native := terminalListenerFixture()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := listener.Accept(t.Context(), time.Second)
	if got.Connection != nil {
		_ = got.Connection.Close()
	}
	if err == nil {
		t.Error("closed original listener acquired a carrier")
	}
	// Join a wrongly created loop too, so the red control leaves no goroutine.
	listener.mu.Lock()
	started := listener.started
	listener.mu.Unlock()
	if started {
		<-listener.acceptDone
	}
	if native.accepts.Load() != 0 {
		t.Fatal("Accept started physical work after completed Close")
	}
}

func TestSharedListenerCanceledWaitCannotTakeQueuedCarrier(t *testing.T) {
	listener, _ := terminalListenerFixture()
	listener.started = true
	listener.timeout = time.Second
	close(listener.acceptDone)
	local, peer := net.Pipe()
	t.Cleanup(func() { _ = local.Close(); _ = peer.Close(); _ = listener.Close() })
	listener.results <- handshakeResult{carrier: transport.ClosedSharedCarrier{Kind: transport.ClosedSharedDirect, Connection: local}}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := listener.Accept(ctx, time.Second)
	if err == nil || got.Connection != nil {
		t.Fatal("canceled wait consumed a previously authenticated carrier")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatal("original cancellation was lost", err)
	}
	// The listener still owns that queued physical carrier. A genuine subsequent
	// waiter can receive it; a refused waiter cannot steal or close another use.
	got, err = listener.Accept(t.Context(), time.Second)
	if err != nil || got.Connection != local {
		t.Fatal("refusal lost the original queued carrier", err)
	}
	if err := got.Connection.Close(); err != nil {
		t.Fatal(err)
	}
}

type heldTerminalConnection struct {
	net.Conn
	entered, release chan struct{}
	cause            error
	once             sync.Once
}

func (c *heldTerminalConnection) Close() error {
	c.once.Do(func() { close(c.entered) })
	<-c.release
	return c.cause
}

func TestSharedListenerCloseJoinsQueuedCarrierAndRetainsLateFailure(t *testing.T) {
	listener, _ := terminalListenerFixture()
	listener.started = true
	listener.timeout = time.Second
	close(listener.acceptDone)
	late := errors.New("original queued carrier close failure")
	conn := &heldTerminalConnection{entered: make(chan struct{}), release: make(chan struct{}), cause: late}
	listener.results <- handshakeResult{carrier: transport.ClosedSharedCarrier{Kind: transport.ClosedSharedDirect, Connection: conn}}
	done := make(chan error, 1)
	go func() { done <- listener.Close() }()
	<-conn.entered
	select {
	case err := <-done:
		t.Fatal("listener returned before physical Close joined", err)
	default:
	}
	close(conn.release)
	if err := <-done; !errors.Is(err, late) {
		t.Fatal("late physical failure was discarded", err)
	}
	if err := listener.Close(); !errors.Is(err, late) {
		t.Fatal("repeated Close lost its retained original result", err)
	}
}
