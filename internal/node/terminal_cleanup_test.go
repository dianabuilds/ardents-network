package node

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestProbeDrainReportsListenerCleanupFailure(t *testing.T) {
	injected := errors.New("injected listener cleanup failure")
	running := &probeListener{plan: &probePlan{config: ProbeConfig{DrainTimeout: time.Second}}, listener: terminalFailingListener{err: injected},
		stop: make(chan struct{}), connections: make(map[net.Conn]struct{})}
	if err := running.drain(context.Background()); !errors.Is(err, injected) {
		t.Fatalf("Drain error = %v, want listener cleanup failure", err)
	}
}

func TestTerminalCleanupRetainsOneBoundedFailure(t *testing.T) {
	first := errors.New("first cleanup failure")
	second := errors.New("second cleanup failure")
	var cleanup terminalCleanup
	cleanup.record(first)
	cleanup.record(second)
	if err := cleanup.result(); !errors.Is(err, first) || errors.Is(err, second) {
		t.Fatalf("cleanup result = %v, want only first failure", err)
	}
}

type terminalFailingListener struct{ err error }

func (terminalFailingListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (listener terminalFailingListener) Close() error     { return listener.err }
func (terminalFailingListener) Addr() net.Addr            { return testAddress("terminal-cleanup") }
