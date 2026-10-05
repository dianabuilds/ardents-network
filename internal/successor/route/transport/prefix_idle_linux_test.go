//go:build linux

package transport

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Idle tests exercise physical lifetime only, with real pipe reader/close
// operations. They supply no Network, admission or prefix readiness authority.
type prefixIdleConn struct {
	net.Conn
	readStarted, readReturned, readGate chan struct{}
	startOnce, returnOnce               sync.Once
	closeFailure                        error
}

func (c *prefixIdleConn) Read(body []byte) (int, error) {
	c.startOnce.Do(func() { close(c.readStarted) })
	n, err := c.Conn.Read(body)
	c.returnOnce.Do(func() { close(c.readReturned) })
	if c.readGate != nil {
		<-c.readGate
	}
	return n, err
}
func (c *prefixIdleConn) Close() error { return errors.Join(c.Conn.Close(), c.closeFailure) }

func idlePhysicalPrefix(t *testing.T, gate chan struct{}, failure error) (*Prefix, *prefixIdleConn, chan time.Time, *atomic.Int32, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	end := time.Now().Add(5 * time.Second)
	entryConn, entryPeer := net.Pipe()
	interiorConn, interiorPeer := net.Pipe()
	entry := &prefixIdleConn{Conn: entryConn, readStarted: make(chan struct{}), readReturned: make(chan struct{}), readGate: gate, closeFailure: failure}
	interior := &prefixIdleConn{Conn: interiorConn, readStarted: make(chan struct{}), readReturned: make(chan struct{})}
	queues := &queueBudget{maximum: 4 << 20}
	p := &Prefix{ctx: ctx, caller: ctx, cancel: cancel, done: make(chan struct{}), closing: make(chan struct{}), entry: newSession(ctx, entry, end, 32<<20, nil, false, queues, nil), interior: newSession(ctx, interior, end, 32<<20, nil, false, queues, nil)}
	released := new(atomic.Int32)
	p.release = func() error {
		for _, reader := range []<-chan struct{}{p.entry.readerDone, p.interior.readerDone} {
			select {
			case <-reader:
			default:
				t.Error("idle release preceded original reader join")
			}
		}
		select {
		case <-p.done:
			t.Error("idle Done preceded resource release")
		default:
		}
		released.Add(1)
		return nil
	}
	idle := make(chan time.Time, 1)
	go p.watch(func() error { return nil }, idle)
	<-entry.readStarted
	<-interior.readStarted
	unblock := func() {
		if gate != nil {
			select {
			case <-gate:
			default:
				close(gate)
			}
		}
	}
	cleanup := func() { unblock(); _ = p.Close(); _ = entryPeer.Close(); _ = interiorPeer.Close(); cancel() }
	return p, entry, idle, released, cleanup
}

func TestPrefixIdleReadinessJoinsAndReleasesBeforeDone(t *testing.T) {
	p, _, idle, released, cleanup := idlePhysicalPrefix(t, nil, nil)
	defer cleanup()
	idle <- time.Now()
	select {
	case <-p.Done():
	case <-time.After(time.Second):
		t.Fatal("idle did not autonomously join before notification")
	}
	if released.Load() != 1 {
		t.Fatal("idle Done preceded autonomous release", released.Load())
	}
	result := p.Close()
	if result != nil || p.Close() != result || released.Load() != 1 {
		t.Fatal("clean idle terminal renewed or failed", result, released.Load())
	}
}

func TestPrefixIdleRetainsLatePhysicalFailureAndConcurrentCloseJoins(t *testing.T) {
	gate := make(chan struct{})
	sentinel := errors.New("idle original Entry physical close failure")
	p, entry, idle, released, cleanup := idlePhysicalPrefix(t, gate, sentinel)
	defer cleanup()
	idle <- time.Now()
	select {
	case <-entry.readReturned:
	case <-time.After(time.Second):
		t.Fatal("idle did not interrupt original physical reader")
	}
	// The original Read has physically returned but remains owned until its
	// result is delivered; both notification and release must wait for that join.
	select {
	case <-p.Done():
		t.Fatal("idle Done published before gated reader joined")
	default:
	}
	if released.Load() != 0 {
		t.Fatal("idle capacity released before original reader join")
	}
	joined := make(chan error, 1)
	go func() { joined <- p.Close() }()
	select {
	case err := <-joined:
		t.Fatal("explicit Close returned before autonomous physical join", err)
	default:
	}
	close(gate)
	select {
	case err := <-joined:
		if !errors.Is(err, sentinel) || p.Close() != err {
			t.Fatal("idle lost/replaced late physical close failure", err)
		}
	case <-time.After(time.Second):
		t.Fatal("concurrent explicit Close deadlocked autonomous idle join")
	}
	select {
	case <-p.Done():
	default:
		t.Fatal("joined Close returned before autonomous notification")
	}
	if released.Load() != 1 {
		t.Fatal("idle joined capacity release count", released.Load())
	}
}
