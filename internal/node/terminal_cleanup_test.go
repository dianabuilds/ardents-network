package node

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestRoleDrainsReportListenerCleanupFailures(t *testing.T) {
	injected := errors.New("injected listener cleanup failure")
	tests := []struct {
		name  string
		drain func() error
	}{
		{name: "Initiator", drain: func() error {
			running := &Initiator{plan: initiatorPlan{InitiatorConfig: InitiatorConfig{DrainTimeout: time.Second}}, listener: terminalFailingListener{err: injected},
				pre: make(map[net.Conn]struct{}), active: make(map[route.Carrier]struct{})}
			return running.Drain(context.Background())
		}},
		{name: "Introduction", drain: func() error {
			running := &Introduction{plan: introductionPlan{IntroductionConfig: IntroductionConfig{DrainTimeout: time.Second}}, listener: terminalFailingListener{err: injected},
				pre: make(map[net.Conn]struct{}), active: make(map[net.Conn]struct{})}
			return running.Drain(context.Background())
		}},
		{name: "Responder", drain: func() error {
			running := &Responder{plan: responderPlan{ResponderConfig: ResponderConfig{DrainTimeout: time.Second}}, listener: terminalFailingListener{err: injected},
				pre: make(map[net.Conn]struct{}), active: make(map[route.Carrier]struct{})}
			return running.Drain(context.Background())
		}},
		{name: "Rendezvous", drain: func() error {
			running := &Rendezvous{plan: rendezvousPlan{RendezvousConfig: RendezvousConfig{DrainTimeout: time.Second}}, listener: terminalFailingCarrierListener{err: injected},
				pre: make(map[route.PendingCarrier]struct{}), waiting: make(map[[32]byte]*rendezvousLeg), active: make(map[route.Carrier]struct{}),
				waitingCap: make(chan struct{}, 1)}
			return running.Drain(context.Background())
		}},
		{name: "probe", drain: func() error {
			running := &probeListener{plan: &probePlan{config: ProbeConfig{DrainTimeout: time.Second}}, listener: terminalFailingListener{err: injected},
				stop: make(chan struct{}), connections: make(map[net.Conn]struct{})}
			return running.drain(context.Background())
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.drain(); !errors.Is(err, injected) {
				t.Fatalf("Drain error = %v, want listener cleanup failure", err)
			}
		})
	}
}

func TestTerminalCleanupPreservesFailureJoinedWithClosed(t *testing.T) {
	injected := errors.New("injected carrier cleanup failure")
	running := &Initiator{plan: initiatorPlan{InitiatorConfig: InitiatorConfig{DrainTimeout: time.Second}}, listener: terminalFailingListener{},
		pre: make(map[net.Conn]struct{}), active: map[route.Carrier]struct{}{terminalFailingCarrier{err: errors.Join(net.ErrClosed, injected)}: {}}}
	if err := running.Drain(context.Background()); !errors.Is(err, injected) {
		t.Fatalf("Drain error = %v, want joined carrier cleanup failure", err)
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

func TestWorkerCleanupFailuresSurviveActiveRemoval(t *testing.T) {
	injected := errors.New("injected worker cleanup failure")
	tests := []struct {
		name string
		run  func() error
	}{
		{name: "Initiator", run: func() error {
			raw := &terminalFailingNetConn{err: injected}
			next := terminalFailingCarrier{err: injected}
			running := &Initiator{plan: initiatorPlan{InitiatorConfig: InitiatorConfig{DrainTimeout: time.Second, RelayByteLimit: 1}}, listener: terminalFailingListener{},
				relays: make(chan struct{}, 1), pre: make(map[net.Conn]struct{}), active: map[route.Carrier]struct{}{raw: {}, next: {}}}
			running.relays <- struct{}{}
			running.work.Add(1)
			running.relay(raw, raw, next)
			return running.Drain(context.Background())
		}},
		{name: "Introduction", run: func() error {
			connection := &terminalFailingNetConn{err: injected}
			reachability := [32]byte{1}
			slot := &introductionLiveSlot{registration: route.IntroductionSlotRegistration{NotAfter: time.Now().Add(time.Hour)}, raw: connection, connection: connection}
			running := &Introduction{plan: introductionPlan{IntroductionConfig: IntroductionConfig{DrainTimeout: time.Second}}, listener: terminalFailingListener{},
				slotsCap: make(chan struct{}, 1), deliveries: make(chan struct{}, 1), pre: make(map[net.Conn]struct{}),
				slots: map[[32]byte]*introductionLiveSlot{reachability: slot}, active: map[net.Conn]struct{}{connection: {}}}
			running.slotsCap <- struct{}{}
			running.deliveries <- struct{}{}
			running.work.Add(1)
			running.forward(slot, reachability, nil)
			return running.Drain(context.Background())
		}},
		{name: "Responder", run: func() error {
			raw := &terminalFailingNetConn{err: injected}
			next := terminalFailingCarrier{err: injected}
			running := &Responder{plan: responderPlan{ResponderConfig: ResponderConfig{DrainTimeout: time.Second, RelayByteLimit: 1}}, listener: terminalFailingListener{},
				relays: make(chan struct{}, 1), pre: make(map[net.Conn]struct{}), active: map[route.Carrier]struct{}{raw: {}, next: {}}}
			running.relays <- struct{}{}
			running.work.Add(1)
			running.relay(raw, raw, next)
			return running.Drain(context.Background())
		}},
		{name: "Rendezvous", run: func() error {
			first := terminalFailingCarrier{err: injected}
			second := terminalFailingCarrier{err: injected}
			running := &Rendezvous{plan: rendezvousPlan{RendezvousConfig: RendezvousConfig{DrainTimeout: time.Second, PairByteLimit: 1}}, listener: terminalFailingCarrierListener{},
				pairs: make(chan struct{}, 1), pre: make(map[route.PendingCarrier]struct{}), waiting: make(map[[32]byte]*rendezvousLeg),
				active: map[route.Carrier]struct{}{first: {}, second: {}}}
			running.pairs <- struct{}{}
			running.work.Add(1)
			running.pump(&rendezvousLeg{connection: first}, &rendezvousLeg{connection: second})
			return running.Drain(context.Background())
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); !errors.Is(err, injected) {
				t.Fatalf("Drain error = %v, want worker cleanup failure", err)
			}
		})
	}
}

type terminalFailingListener struct{ err error }

func (terminalFailingListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (listener terminalFailingListener) Close() error     { return listener.err }
func (terminalFailingListener) Addr() net.Addr            { return testAddress("terminal-cleanup") }

type terminalFailingCarrierListener struct{ err error }

func (terminalFailingCarrierListener) Accept(context.Context) (route.PendingCarrier, error) {
	return nil, net.ErrClosed
}
func (listener terminalFailingCarrierListener) Close() error { return listener.err }

type terminalFailingCarrier struct{ err error }

func (terminalFailingCarrier) Read([]byte) (int, error)    { return 0, net.ErrClosed }
func (terminalFailingCarrier) Write([]byte) (int, error)   { return 0, net.ErrClosed }
func (terminalFailingCarrier) SetDeadline(time.Time) error { return nil }
func (carrier terminalFailingCarrier) Close() error        { return carrier.err }

type terminalFailingNetConn struct{ err error }

func (*terminalFailingNetConn) Read([]byte) (int, error)         { return 0, net.ErrClosed }
func (*terminalFailingNetConn) Write([]byte) (int, error)        { return 0, net.ErrClosed }
func (connection *terminalFailingNetConn) Close() error          { return connection.err }
func (*terminalFailingNetConn) LocalAddr() net.Addr              { return testAddress("worker-local") }
func (*terminalFailingNetConn) RemoteAddr() net.Addr             { return testAddress("worker-remote") }
func (*terminalFailingNetConn) SetDeadline(time.Time) error      { return nil }
func (*terminalFailingNetConn) SetReadDeadline(time.Time) error  { return nil }
func (*terminalFailingNetConn) SetWriteDeadline(time.Time) error { return nil }
