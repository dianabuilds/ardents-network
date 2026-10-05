//go:build linux

package receiver

import (
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/join"
)

// This fixture exercises the public receiving Pairing interface so the actual
// Receiver.record result can be checked alongside physical completion.
// It supplies no successful Network, Admission, Hosting or Service authority.
type joinBoundsPeer struct {
	conn     net.Conn
	done     chan error
	capacity *join.Capacity
	finished bool
	result   error
}

type joinBoundsFixture struct {
	ctx    context.Context
	cancel context.CancelFunc
	pairs  *join.Pairing
	queues *framing.Budget
	peers  []*joinBoundsPeer
}

func newJoinBoundsFixture() *joinBoundsFixture {
	ctx, cancel := context.WithCancel(context.Background())
	return &joinBoundsFixture{ctx: ctx, cancel: cancel, queues: framing.NewBudget(64 << 20), pairs: join.NewPairing(nil)}
}

func (f *joinBoundsFixture) start(t *testing.T, request ardp.JoinRequest, wrap func(net.Conn) net.Conn) *joinBoundsPeer {
	return f.startWithProfile(t, request, wrap, [32]byte{3})
}

func (f *joinBoundsFixture) startWithProfile(t *testing.T, request ardp.JoinRequest, wrap func(net.Conn) net.Conn, profile [32]byte) *joinBoundsPeer {
	return f.startWithBounds(t, request, wrap, profile, 32<<20)
}

func (f *joinBoundsFixture) startWithBounds(t *testing.T, request ardp.JoinRequest, wrap func(net.Conn) net.Conn, profile [32]byte, limit uint64) *joinBoundsPeer {
	t.Helper()
	local, remote := net.Pipe()
	capacity, err := join.ReserveCapacity(f.queues)
	if err != nil {
		_ = local.Close()
		_ = remote.Close()
		t.Fatal(err)
	}
	peer := &joinBoundsPeer{conn: remote, done: make(chan error, 1), capacity: capacity}
	f.peers = append(f.peers, peer)
	if wrap != nil {
		local = wrap(local)
	}
	hello := ardp.Hello{ProfileDigest: profile, Purpose: ardp.PurposeDataJoin, Deadline: time.Now().Add(time.Minute).UTC().Truncate(time.Second)}
	go func() { peer.done <- f.pairs.Serve(f.ctx, local, hello, limit, nil, capacity) }()
	body, err := ardp.EncodeJoinRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindOperation, Lane: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	return peer
}

func (p *joinBoundsPeer) wait() error {
	if !p.finished {
		p.result = <-p.done
		p.finished = true
	}
	return p.result
}

func (f *joinBoundsFixture) close(t *testing.T) {
	t.Helper()
	f.cancel()
	for _, peer := range f.peers {
		_ = peer.conn.Close()
	}
	for _, peer := range f.peers {
		_ = peer.wait()
		peer.capacity.Release()
	}
	assertRetainedBudget(t, f.queues, 64<<20, 0, 0)
}

func joinBoundsRequest(side, nonce uint8) ardp.JoinRequest {
	return ardp.JoinRequest{Nonce: [32]byte{nonce}, Secret: [32]byte{4}, Context: [32]byte{5}, Side: side, Deadline: time.Now().Add(10 * time.Second).UTC().Truncate(time.Second)}
}

func readJoinBoundsResult(t *testing.T, peer *joinBoundsPeer, nonce uint8) {
	t.Helper()
	frame, err := ardp.ReadFrame(peer.conn)
	if err != nil {
		t.Fatal(err)
	}
	if frame.Kind != ardp.KindResult || frame.Lane != 1 {
		t.Fatal("paired side did not receive its local RESULT")
	}
	if status, err := ardp.DecodeJoinResult(frame.Body, [32]byte{nonce}); err != nil || status != 0 {
		t.Fatal("original side or accepted result changed", err)
	}
}

// These controls cover physical failure retention across join.Pairing setup,
// joinPair barrier/relay/termination and Receiver.record. They are organized by
// the retained receiving result, not by one particular frame or source file.
// This injects one physical deadline failure; later interruption and Close use
// the real connection. It supplies no successful authority or Admission.
type joinDeadlineFault struct {
	net.Conn
	cause     error
	writeOnly bool
	armed     atomic.Bool
	fired     atomic.Bool
}

func (c *joinDeadlineFault) SetDeadline(end time.Time) error {
	if !c.writeOnly && c.armed.CompareAndSwap(true, false) {
		c.fired.Store(true)
		return c.cause
	}
	return c.Conn.SetDeadline(end)
}

func (c *joinDeadlineFault) SetWriteDeadline(end time.Time) error {
	if c.writeOnly && c.armed.CompareAndSwap(true, false) {
		c.fired.Store(true)
		return c.cause
	}
	return c.Conn.SetWriteDeadline(end)
}

func assertJoinDeadlineRetained(t *testing.T, receiver *Receiver, fault *joinDeadlineFault, peers ...*joinBoundsPeer) {
	t.Helper()
	for _, peer := range peers {
		if err := peer.wait(); !errors.Is(err, fault.cause) {
			t.Fatal("JOIN handler lost the injected deadline failure", err)
		}
	}
	if !fault.fired.Load() {
		t.Fatal("physical deadline failure was not exercised")
	}
	receiver.mu.Lock()
	retained := receiver.err
	receiver.mu.Unlock()
	if !errors.Is(retained, fault.cause) {
		t.Fatal("Receiver.record lost the joined physical deadline failure", retained)
	}
}

func TestJoinInitialDeadlineFailureReachesReceiverTerminal(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newJoinBoundsFixture()
		defer fixture.close(t)
		receiver := &Receiver{}
		// Listen installs this exact callback for its receiving JOIN owner.
		fixture.pairs = join.NewPairing(receiver.record)
		local, remote := net.Pipe()
		capacity, err := join.ReserveCapacity(fixture.queues)
		if err != nil {
			_ = local.Close()
			_ = remote.Close()
			t.Fatal(err)
		}
		peer := &joinBoundsPeer{conn: remote, done: make(chan error, 1), capacity: capacity}
		fixture.peers = append(fixture.peers, peer)
		fault := &joinDeadlineFault{Conn: local, cause: errors.New("initial JOIN deadline failed")}
		fault.armed.Store(true)
		hello := ardp.Hello{ProfileDigest: [32]byte{3}, Purpose: ardp.PurposeDataJoin, Deadline: time.Now().Add(time.Minute).UTC().Truncate(time.Second)}
		go func() { peer.done <- fixture.pairs.Serve(fixture.ctx, fault, hello, 32<<20, nil, capacity) }()
		assertJoinDeadlineRetained(t, receiver, fault, peer)
	})
}

func TestJoinPairedDeadlineFailureReachesReceiverTerminal(t *testing.T) {
	for _, phase := range []string{"barrier", "data-deadline-reset", "relay-interruption", "terminal-write-deadline"} {
		t.Run(phase, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				fixture := newJoinBoundsFixture()
				defer fixture.close(t)
				receiver := &Receiver{}
				fixture.pairs = join.NewPairing(receiver.record)
				var fault *joinDeadlineFault
				first := fixture.start(t, joinBoundsRequest(1, 1), func(conn net.Conn) net.Conn {
					fault = &joinDeadlineFault{Conn: conn, cause: errors.New("JOIN " + phase + " failed"), writeOnly: phase == "terminal-write-deadline"}
					return fault
				})
				if phase == "barrier" {
					// The first request is waiting. The next physical deadline on
					// that side belongs to its RESULT writer after the real match.
					fault.armed.Store(true)
				}
				second := fixture.start(t, joinBoundsRequest(2, 2), nil)
				if phase != "barrier" {
					readJoinBoundsResult(t, first, 1)
					if phase == "data-deadline-reset" {
						// The unread second RESULT still holds the barrier. Releasing
						// it makes the next deadline restore the original data bound.
						fault.armed.Store(true)
					}
					readJoinBoundsResult(t, second, 2)
				}
				if phase == "relay-interruption" || phase == "terminal-write-deadline" {
					synctest.Wait()
					fault.armed.Store(true)
					if err := ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
						t.Fatal(err)
					}
					if phase == "terminal-write-deadline" {
						// Drain a completed opposite terminal, or physical retirement.
						// An unread healthy write must not add a timeout failure.
						frame, err := ardp.ReadFrame(second.conn)
						if err != nil && err != io.EOF || err == nil && (frame.Kind != ardp.KindClose || frame.Lane != 1 || len(frame.Body) != 1) {
							t.Fatal("opposite terminal framing failed", err)
						}
					}
				}
				assertJoinDeadlineRetained(t, receiver, fault, first, second)
			})
		})
	}
}

// A real prefix of the first terminal frame reaches the peer before this
// wrapper returns the physical failure. Other I/O remains the actual pipe.
type joinTerminalWriteFault struct {
	net.Conn
	cause error
	fired atomic.Bool
}

func (c *joinTerminalWriteFault) Write(raw []byte) (int, error) {
	if len(raw) >= ardp.HeaderSize && raw[6] == ardp.KindClose && c.fired.CompareAndSwap(false, true) {
		n, err := c.Conn.Write(raw[:8])
		if err != nil {
			return n, err
		}
		return n, c.cause
	}
	return c.Conn.Write(raw)
}

func TestJoinPartialTerminalFailurePreventsLaterTerminalOutput(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newJoinBoundsFixture()
		defer fixture.close(t)
		receiver := &Receiver{}
		fixture.pairs = join.NewPairing(receiver.record)
		failure := errors.New("partially emitted JOIN terminal failed")
		var fault *joinTerminalWriteFault
		first := fixture.start(t, joinBoundsRequest(1, 1), func(conn net.Conn) net.Conn {
			fault = &joinTerminalWriteFault{Conn: conn, cause: failure}
			return fault
		})
		second := fixture.start(t, joinBoundsRequest(2, 2), nil)
		readJoinBoundsResult(t, first, 1)
		readJoinBoundsResult(t, second, 2)
		synctest.Wait()
		if err := ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
			t.Fatal(err)
		}
		var prefix [8]byte
		if _, err := io.ReadFull(first.conn, prefix[:]); err != nil || prefix[6] != ardp.KindClose {
			t.Fatal("terminal frame did not physically start before failure", err)
		}
		frame, err := ardp.ReadFrame(second.conn)
		if err == nil {
			t.Errorf("opposite terminal output followed a partially failed CLOSE: kind=%d body=%v", frame.Kind, frame.Body)
		} else if err != io.EOF {
			t.Error("opposite physical retirement returned an unexpected framing error", err)
		}
		for _, peer := range []*joinBoundsPeer{first, second} {
			if err := peer.wait(); !errors.Is(err, failure) || !containsPhysicalWrite(err) {
				t.Error("joined handler lost its terminal physical failure", err)
			}
		}
		if !fault.fired.Load() {
			t.Fatal("terminal write failure was not exercised")
		}
		receiver.mu.Lock()
		retained := receiver.err
		receiver.mu.Unlock()
		if !errors.Is(retained, failure) {
			t.Fatal("Receiver.record lost the partially failed terminal", retained)
		}
	})
}

// Probe both finite resources through the same claims real borrowers use.
// Filling the expected remainder detects retained memory in either direction;
// zero-memory claims independently detect an early or leaked child return.
func assertRetainedBudget(t *testing.T, budget *framing.Budget, maximum, used uint64, children int) {
	t.Helper()
	remainder, err := budget.HoldChild(maximum - used)
	if err != nil {
		t.Fatal("expected remaining principal memory unavailable", err)
	}
	defer remainder()
	if extra, err := budget.HoldChild(1); err == nil {
		extra()
		t.Fatal("principal memory returned before its owner joined")
	}
	var claims []func()
	defer func() {
		for _, release := range claims {
			release()
		}
	}()
	for range 1023 - children {
		release, err := budget.HoldChild(0)
		if err != nil {
			t.Fatal("expected remaining child capacity unavailable", err)
		}
		claims = append(claims, release)
	}
	if extra, err := budget.HoldChild(0); err == nil {
		extra()
		t.Fatal("child returned before its owner joined")
	}
}

func containsPhysicalWrite(err error) bool {
	stage := framing.TerminalFailureStage(err)
	if stage == "physical-write" || stage == "peer-retired-write" {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if containsPhysicalWrite(child) {
				return true
			}
		}
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return containsPhysicalWrite(wrapped.Unwrap())
	}
	return false
}
