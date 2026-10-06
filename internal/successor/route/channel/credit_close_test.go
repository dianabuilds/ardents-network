package channel

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// This wrapper emits an actual physical prefix of CREDIT. Gates retain the
// remainder or a late physical result; no successful authority is supplied.
type creditCloseConn struct {
	net.Conn
	emitted, restGate, returned, completion chan struct{}
	once                                    sync.Once
	started                                 atomic.Bool
	mu                                      sync.Mutex
	deadlines                               []time.Time
	deadlineFailure, writeFailure           error
}

func (c *creditCloseConn) Write(raw []byte) (int, error) {
	if len(raw) < ardp.HeaderSize || raw[6] != ardp.KindCredit || !c.started.CompareAndSwap(false, true) {
		return c.Conn.Write(raw)
	}
	n, err := c.Conn.Write(raw[:8])
	if err != nil {
		return n, err
	}
	c.once.Do(func() { close(c.emitted) })
	if c.restGate != nil {
		<-c.restGate
	}
	rest, err := c.Conn.Write(raw[8:])
	n += rest
	if c.returned != nil {
		close(c.returned)
	}
	if c.completion != nil {
		<-c.completion
	}
	return n, errors.Join(err, c.writeFailure)
}

func (c *creditCloseConn) SetWriteDeadline(end time.Time) error {
	select {
	case <-c.emitted:
		c.mu.Lock()
		c.deadlines = append(c.deadlines, end)
		c.mu.Unlock()
		err := c.Conn.SetWriteDeadline(end)
		return errors.Join(err, c.deadlineFailure)
	default:
		return c.Conn.SetWriteDeadline(end)
	}
}

type creditCloseFixture struct {
	s        *Session
	l        *Lane
	physical *creditCloseConn
	peer     net.Conn
	queues   *Budget
	prefix   [8]byte
	consumed chan error
	release  func()
}

func newCreditCloseFixture(t *testing.T, original time.Duration, wrap func(*creditCloseConn)) *creditCloseFixture {
	t.Helper()
	local, peer := net.Pipe()
	physical := &creditCloseConn{Conn: local, emitted: make(chan struct{})}
	if wrap != nil {
		wrap(physical)
	}
	queues := &Budget{maximum: 4 << 20}
	release, err := queues.HoldControl()
	if err != nil {
		t.Fatal(err)
	}
	s, l, err := newJoinedSession(context.Background(), physical, time.Now().Add(10*time.Second), 1<<20, nil, queues)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	l.end = time.Now().Add(original)
	l.readEnd = l.end
	l.writeEnd = l.end
	s.mu.Unlock()
	f := &creditCloseFixture{s: s, l: l, physical: physical, peer: peer, queues: queues, consumed: make(chan error, 1), release: release}
	// The ordinary reader reserves and queues a genuine input BYTES frame;
	// actual consumption generates the single-byte CREDIT being tested.
	if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{42}}); err != nil {
		t.Fatal(err)
	}
	go func() {
		var value [1]byte
		n, err := l.Read(value[:])
		if n != 1 || value[0] != 42 {
			err = errors.Join(err, errors.New("CREDIT consumption differs"))
		}
		f.consumed <- err
	}()
	if _, err := io.ReadFull(peer, f.prefix[:]); err != nil {
		t.Fatal(err)
	}
	<-physical.emitted
	return f
}

func (f *creditCloseFixture) readCredit(t *testing.T) {
	t.Helper()
	frame, err := ardp.ReadFrame(io.MultiReader(bytes.NewReader(f.prefix[:]), f.peer))
	if err != nil || frame.Kind != ardp.KindCredit || frame.Lane != 1 || !bytes.Equal(frame.Body, []byte{0, 0, 0, 1}) {
		t.Fatalf("already-emitted CREDIT did not finish: %+v / %v", frame, err)
	}
}

func (f *creditCloseFixture) close() { _ = f.peer.Close(); _ = f.s.Close(); f.release() }

func TestCREDITStartedKeepsOriginalBoundWhenDataWriteDeadlineExpires(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newCreditCloseFixture(t, 5*time.Second, nil)
		defer f.close()
		// TLS CloseWrite sets its underlying data-write deadline to now. The
		// already started receive-credit frame has its own original lane bound.
		if err := f.l.SetWriteDeadline(time.Now()); err != nil {
			t.Fatal(err)
		}
		f.physical.mu.Lock()
		deadlines := append([]time.Time(nil), f.physical.deadlines...)
		f.physical.mu.Unlock()
		if len(deadlines) != 0 {
			t.Fatal("data-only deadline interrupted physical CREDIT", deadlines)
		}
		f.readCredit(t)
		if err := <-f.consumed; err != nil {
			t.Fatal("receive credit failed after data half-close", err)
		}
		if n, err := f.l.Write([]byte{1}); n != 0 || !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatal("expired data output was renewed", n, err)
		}
		if !f.s.Live() || f.s.PhysicalFailure() != nil {
			t.Fatal("data half-close poisoned its framing owner")
		}
	})
}

func TestCREDITEmittedCloseGraceAllowsPhysicalCompletion(t *testing.T) {
	for _, mode := range []string{"local", "peer"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := newCreditCloseFixture(t, 5*time.Second, nil)
				defer f.close()
				closing := make(chan error, 1)
				if mode == "local" {
					go func() { closing <- f.l.Close() }()
				} else {
					if err := ardp.WriteFrame(f.peer, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
						t.Fatal(err)
					}
					<-f.s.readerDone
				}
				synctest.Wait()
				f.physical.mu.Lock()
				deadlines := append([]time.Time(nil), f.physical.deadlines...)
				f.physical.mu.Unlock()
				if len(deadlines) == 0 || !deadlines[0].After(time.Now()) {
					t.Fatalf("emitted CREDIT interrupted instead of bounded cleanup grace: %v", deadlines)
				}
				f.readCredit(t)
				if err := <-f.consumed; err != nil {
					t.Fatal("permitted emitted CREDIT failed", err)
				}
				if mode == "local" {
					frame, err := ardp.ReadFrame(f.peer)
					if err != nil || frame.Kind != ardp.KindClose {
						t.Fatal("local terminal did not follow completed CREDIT", err)
					}
					if err := <-closing; err != nil {
						t.Fatal(err)
					}
				}
				if err := f.s.Close(); err != nil {
					t.Fatal("successful control completion poisoned framing", err)
				}
			})
		})
	}
}

func TestCREDITCloseGraceCannotExtendOriginalOrFirstCleanupBound(t *testing.T) {
	for _, original := range []time.Duration{250 * time.Millisecond, 5 * time.Second} {
		t.Run(original.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				started := time.Now()
				f := newCreditCloseFixture(t, original, nil)
				defer f.close()
				if err := ardp.WriteFrame(f.peer, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
					t.Fatal(err)
				}
				<-f.s.readerDone
				expected := started.Add(time.Second)
				if original < time.Second {
					expected = started.Add(original)
				}
				f.physical.mu.Lock()
				initial := append([]time.Time(nil), f.physical.deadlines...)
				f.physical.mu.Unlock()
				if len(initial) != 1 || initial[0] != expected {
					t.Fatalf("first CREDIT cleanup deadline=%v want %s", initial, expected)
				}
				time.Sleep(100 * time.Millisecond)
				if err := f.l.SetWriteDeadline(started.Add(9 * time.Second)); err != nil {
					t.Fatal(err)
				}
				closed := make(chan error, 1)
				go func() { closed <- f.l.Close() }()
				synctest.Wait()
				select {
				case err := <-closed:
					t.Fatalf("peer=true local Close returned before accepted CREDIT joined: %v", err)
				default:
				}
				f.physical.mu.Lock()
				deadlines := append([]time.Time(nil), f.physical.deadlines...)
				f.physical.mu.Unlock()
				for _, deadline := range deadlines {
					if deadline.After(expected) {
						t.Fatalf("later operation extended immutable CREDIT cleanup bound: %s > %s", deadline, expected)
					}
				}
				// Do not consume the remaining frame. Its actual pipe write must fail at
				// the retained earlier bound, never become a clean terminal outcome.
				if err := <-f.consumed; !errors.Is(err, os.ErrDeadlineExceeded) {
					t.Fatal("missing original bounded physical timeout", err)
				}
				if time.Now() != expected {
					t.Fatalf("physical CREDIT ended at %s want %s", time.Now(), expected)
				}
				<-closed
				if err := f.s.Close(); !errors.Is(err, os.ErrDeadlineExceeded) {
					t.Fatal("bounded CREDIT timeout disappeared from terminal", err)
				}
			})
		})
	}
}

func TestCREDITPeerClosedLocalCloseJoinsLatePhysicalFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		late := errors.New("late actual CREDIT writer failure")
		completion := make(chan struct{})
		var once sync.Once
		unblock := func() { once.Do(func() { close(completion) }) }
		defer unblock()
		f := newCreditCloseFixture(t, 5*time.Second, func(c *creditCloseConn) {
			c.returned = make(chan struct{})
			c.completion = completion
			c.writeFailure = late
		})
		defer func() { unblock(); f.close() }()
		if err := ardp.WriteFrame(f.peer, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
			t.Fatal(err)
		}
		<-f.s.readerDone
		closed := make(chan error, 1)
		go func() { closed <- f.l.Close() }()
		<-f.physical.returned
		select {
		case err := <-closed:
			t.Fatalf("peer=true local Close did not join accepted late CREDIT writer: %v", err)
		default:
		}
		joined := make(chan error, 1)
		go func() { err := f.s.Close(); f.release(); joined <- err }()
		synctest.Wait()
		select {
		case err := <-joined:
			t.Fatalf("session returned before CREDIT result joined: %v", err)
		default:
		}
		f.queues.mu.Lock()
		held := f.queues.used
		f.queues.mu.Unlock()
		if held < 16<<10 {
			t.Fatal("original control capacity returned before physical join", held)
		}
		unblock()
		if err := <-f.consumed; !errors.Is(err, late) {
			t.Fatal("late CREDIT writer cause lost", err)
		}
		<-closed
		result := <-joined
		if !errors.Is(result, late) || f.s.Close() != result {
			t.Fatal("joined terminal lost/replaced late CREDIT failure", result)
		}
		var failure *physicalWriteFailure
		if !errors.As(f.s.PhysicalFailure(), &failure) || failure.kind != ardp.KindCredit || !errors.Is(failure, late) {
			t.Fatal("late CREDIT physical provenance lost", f.s.PhysicalFailure())
		}
		f.queues.mu.Lock()
		held = f.queues.used
		children := f.queues.children
		f.queues.mu.Unlock()
		if held != 0 || children != 0 {
			t.Fatal("joined CREDIT capacity leaked", held, children)
		}
	})
}

func TestCREDITCloseRetainsDeadlineOperationFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sentinel := errors.New("CREDIT cleanup deadline operation failed")
		f := newCreditCloseFixture(t, 5*time.Second, func(c *creditCloseConn) { c.deadlineFailure = sentinel })
		defer f.close()
		closed := make(chan error, 1)
		go func() { closed <- f.l.Close() }()
		synctest.Wait()
		// Closing the peer guarantees actual physical return even if interruption
		// failed. That external return cannot erase the owned deadline failure.
		_ = f.peer.Close()
		<-f.consumed
		<-closed
		if err := f.s.Close(); !errors.Is(err, sentinel) {
			t.Fatal("cleanup deadline-operation sentinel lost", err)
		}
	})
}
