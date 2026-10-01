package probe

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestProbeDrainReportsListenerCleanupFailure(t *testing.T) {
	injected := errors.New("injected listener cleanup failure")
	running := &probeListener{plan: &Plan{config: Config{DrainTimeout: time.Second}}, listener: terminalFailingListener{err: injected},
		stop: make(chan struct{}), connections: make(map[net.Conn]struct{})}
	if err := running.drain(context.Background()); !errors.Is(err, injected) {
		t.Fatalf("Drain error = %v, want listener cleanup failure", err)
	}
}

type terminalFailingListener struct{ err error }

func (terminalFailingListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (listener terminalFailingListener) Close() error     { return listener.err }
func (terminalFailingListener) Addr() net.Addr            { return testAddress("terminal-cleanup") }

func TestListenerFailureIsTerminal(t *testing.T) {
	want := errors.New("listener failed")
	server := &probeListener{listener: failingListener{err: want}, open: make(chan struct{}, 1),
		stop: make(chan struct{}), terminal: make(chan error, 1), connections: make(map[net.Conn]struct{})}
	server.work.Add(1)
	go server.accept()
	select {
	case got := <-server.terminal:
		if !errors.Is(got, want) {
			t.Fatalf("terminal error = %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("listener failure was hidden")
	}
	server.work.Wait()
}

type failingListener struct{ err error }

func (listener failingListener) Accept() (net.Conn, error) { return nil, listener.err }
func (failingListener) Close() error                       { return nil }
func (failingListener) Addr() net.Addr                     { return testAddress("probe") }

type testAddress string

func (address testAddress) Network() string { return string(address) }
func (address testAddress) String() string  { return string(address) }
