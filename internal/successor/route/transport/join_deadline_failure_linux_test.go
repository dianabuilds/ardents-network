//go:build linux

package transport

import (
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

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
		fixture.pairs.record = receiver.record
		local, remote := net.Pipe()
		capacity, err := reserveJoinCapacity(&fixture.queues)
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
		go func() { peer.done <- fixture.pairs.serve(fixture.ctx, fault, hello, 32<<20, nil, capacity) }()
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
				fixture.pairs.record = receiver.record
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
