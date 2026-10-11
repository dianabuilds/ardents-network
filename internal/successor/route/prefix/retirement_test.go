package prefix

import (
	"context"
	"errors"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
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
	queues := framing.NewBudget(4 << 20)
	p := &Prefix{ctx: ctx, caller: ctx, cancel: cancel, done: make(chan struct{}), closing: make(chan struct{}), entry: framing.New(ctx, entry, end, 32<<20, nil, false, queues, nil), interior: framing.New(ctx, interior, end, 32<<20, nil, false, queues, nil)}
	p.config.Deadline = end
	released := new(atomic.Int32)
	p.release = func() error {
		for _, reader := range []<-chan struct{}{p.entry.Done(), p.interior.Done()} {
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

// The virtual clock and manually delivered idle event isolate physical loan
// retention; they do not qualify the real300+60 Publication timing boundary.
func TestBoundedLifetimeSurvivesQuietIdleThenJoinsOriginalUsers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, entry, idle, released, cleanup := idlePhysicalPrefix(t, nil, nil)
		defer cleanup()
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()
		interrupted, userJoined := make(chan struct{}), make(chan struct{})
		defer func() {
			select {
			case <-userJoined:
			default:
				close(userJoined)
			}
		}()
		var once sync.Once
		late := errors.New("original quiet operation join failed")
		var borrow *Borrow
		var err error
		borrow, err = p.BorrowLifetime(ctx, func() { once.Do(func() { close(interrupted) }) }, func() error {
			<-userJoined
			borrow.ReturnJoined()
			return late
		})
		if err != nil {
			t.Fatal(err)
		}
		idle <- time.Now()
		synctest.Wait()
		select {
		case <-p.Done():
			t.Fatal("quiet bounded operation lost its original physical prefix")
		default:
		}
		select {
		case <-interrupted:
			t.Fatal("idle revoked a live bounded operation")
		default:
		}
		if released.Load() != 0 {
			t.Fatal("quiet lifetime returned resources")
		}
		cancel()
		idle <- time.Now()
		synctest.Wait()
		select {
		case <-interrupted:
		default:
			t.Fatal("retirement failed to synchronously interrupt original users")
		}
		select {
		case <-entry.readReturned:
			t.Fatal("physical parent retired before original users joined")
		default:
		}
		close(userJoined)
		synctest.Wait()
		if err := p.Close(); !errors.Is(err, late) || p.Close() != err || released.Load() != 1 {
			t.Fatal("quiet lifetime lost its joined failure or resource ownership", err, released.Load())
		}
	})
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
