package channel

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

type joinedCloseWriteObservation struct {
	net.Conn
	started    chan struct{}
	once       sync.Once
	active     atomic.Bool
	interrupts atomic.Int32
}

// Mechanical framing fixtures start reading immediately; production installs
// its lane with prepareJoinedSession and starts only at its genuine handoff.
func newJoinedSession(ctx context.Context, conn net.Conn, end time.Time, remaining uint64, check func() error, queues *Budget) (*Session, *Lane, error) {
	s, l, err := PrepareJoined(ctx, conn, end, remaining, check, queues)
	if err != nil {
		return nil, nil, err
	}
	s.Start()
	return s, l, nil
}

func (c *joinedCloseWriteObservation) Write(body []byte) (int, error) {
	c.active.Store(true)
	c.once.Do(func() { close(c.started) })
	return c.Conn.Write(body)
}

func (c *joinedCloseWriteObservation) SetWriteDeadline(end time.Time) error {
	if c.active.Load() && !end.IsZero() && !end.After(time.Now()) {
		c.interrupts.Add(1)
	}
	return c.Conn.SetWriteDeadline(end)
}

func TestDedicatedPeerCloseDoesNotInterruptSelectedLocalClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		local, remote := net.Pipe()
		defer remote.Close()
		physical := &joinedCloseWriteObservation{Conn: local, started: make(chan struct{})}
		queues := &Budget{maximum: 4 << 20}
		s, l, err := newJoinedSession(context.Background(), physical, time.Now().Add(time.Minute), 1<<20, nil, queues)
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		sent := make(chan error, 1)
		go func() { sent <- l.Close() }()
		<-physical.started
		synctest.Wait()
		s.mu.Lock()
		selected := s.active == l && s.activeKind == ardp.KindClose
		s.mu.Unlock()
		if !selected {
			t.Fatal("local terminal did not enter actual blocked physical output")
		}
		// Opposite directions progress independently. Do not consume the local
		// CLOSE until its peer's canonical inner CLOSE has reached the reader.
		if err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
			t.Fatal(err)
		}
		<-s.readerDone
		if physical.interrupts.Load() != 0 {
			writerErr := <-sent
			t.Fatalf("peer CLOSE interrupted selected valid local CLOSE: deadline interrupts=%d writer=%v", physical.interrupts.Load(), writerErr)
		}
		select {
		case err := <-sent:
			t.Fatalf("local CLOSE finished before physical peer consumption: %v", err)
		default:
		}
		frame, err := ardp.ReadFrame(remote)
		if err != nil || frame.Kind != ardp.KindClose || frame.Lane != 1 || len(frame.Body) != 1 || frame.Body[0] != 0 {
			t.Fatalf("selected local terminal did not finish: %+v / %v", frame, err)
		}
		if err := <-sent; err != nil {
			t.Fatal("valid terminal writer failed", err)
		}
		if err := s.Close(); err != nil {
			t.Fatal("joined terminal retained a spurious write failure", err)
		}
		if queues.children != 0 || queues.used != 0 {
			t.Fatal("terminal writer was not joined before framing capacity return")
		}
	})
}

// These controls enter the framing owner after its acceptance boundary. Pipe
// frames exercise physical reader/writer ordering, not TLS authentication,
// Network authority, Admission spend or a genuine successful JOIN handshake.
func TestDedicatedLocalCloseStillRequiresPeerInnerClose(t *testing.T) {
	for _, terminal := range []string{"accepted", "refused", "raw-EOF"} {
		t.Run(terminal, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				local, remote := net.Pipe()
				defer remote.Close()
				queues := &Budget{maximum: 4 << 20}
				s, l, err := newJoinedSession(context.Background(), local, time.Now().Add(time.Minute), 1<<20, nil, queues)
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close()
				sent := make(chan error, 1)
				go func() { sent <- l.Close() }()
				frame, err := ardp.ReadFrame(remote)
				if err != nil || frame.Kind != ardp.KindClose || frame.Lane != 1 || len(frame.Body) != 1 || frame.Body[0] != 0 {
					t.Fatalf("local terminal frame differs: %+v / %v", frame, err)
				}
				if err := <-sent; err != nil {
					t.Fatal(err)
				}
				s.mu.Lock()
				locallyClosed := l.closed && l.localClosed && !l.peerClosed
				s.mu.Unlock()
				if !locallyClosed {
					t.Fatal("test did not enter locally closed lane before peer terminal")
				}
				if terminal != "raw-EOF" {
					status := byte(0)
					if terminal == "refused" {
						status = 1
					}
					if err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{status}}); err != nil {
						t.Fatal(err)
					}
				}
				// Transport EOF immediately follows the peer terminal. A discarded inner
				// CLOSE therefore fails causally rather than waiting until an outer timeout.
				if err := remote.Close(); err != nil {
					t.Fatal(err)
				}
				<-s.readerDone
				s.mu.Lock()
				peerClosed, peerRefused := l.peerClosed, l.peerRefused
				s.mu.Unlock()
				joined := s.Close()
				switch terminal {
				case "accepted":
					if !peerClosed || peerRefused || joined != nil {
						t.Fatalf("local CLOSE discarded clean peer inner CLOSE: seen=%v refused=%v joined=%v", peerClosed, peerRefused, joined)
					}
				case "refused":
					if !peerClosed || !peerRefused || joined == nil || errors.Is(joined, io.ErrUnexpectedEOF) {
						t.Fatalf("peer refusal provenance discarded: seen=%v refused=%v joined=%v", peerClosed, peerRefused, joined)
					}
				case "raw-EOF":
					if peerClosed || !errors.Is(joined, io.ErrUnexpectedEOF) {
						t.Fatalf("raw EOF became clean inner termination: seen=%v joined=%v", peerClosed, joined)
					}
				}
				if queues.children != 0 || queues.used != 0 {
					t.Fatal("joined terminal retained framing capacity")
				}
			})
		})
	}
}
