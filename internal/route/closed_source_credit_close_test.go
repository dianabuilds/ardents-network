//go:build linux

package route

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The upper CLOSE has reached Write but no lower output has begun. A genuine
// concurrent CREDIT completes before the lower peer closes successfully.
type sourcePayloadBoundary struct {
	net.Conn
	armed            atomic.Bool
	entered, release chan struct{}
}

func (connection *sourcePayloadBoundary) Write(value []byte) (int, error) {
	if connection.armed.CompareAndSwap(true, false) {
		close(connection.entered)
		<-connection.release
	}
	return connection.Conn.Write(value)
}

func TestClosedSourceUnwrittenCloseSurvivesSuccessfulConcurrentCredit(t *testing.T) {
	checkSourceConcurrentCredit(t, false)
}

func TestClosedSourceUnwrittenCloseRetainsFailedConcurrentCredit(t *testing.T) {
	checkSourceConcurrentCredit(t, true)
}

func checkSourceConcurrentCredit(t *testing.T, partial bool) {
	t.Helper()
	var helpers sync.WaitGroup
	local, peer := net.Pipe()
	end := time.Now().UTC().Add(10 * time.Second).Truncate(time.Second)
	retirement := &closedRoleRetirement{transport: local}
	prefix := &ClosedSourcePrefix{retirement: retirement, stop: func() bool { return true }, interrupted: make(chan struct{}), done: make(chan struct{})}
	child := prefix.newChild(local, end)
	held := &sourcePayloadBoundary{Conn: child, entered: make(chan struct{}), release: make(chan struct{})}
	prefix.connection, prefix.child = held, child
	prefix.channels = newClosedSourceChannelOwner(held, end, retirement.close)
	prefix.channels.framing = child
	prefix.channels.start()
	go prefix.finishAfterChannels()
	var unblock sync.Once
	t.Cleanup(func() {
		defer helpers.Wait()
		unblock.Do(func() { close(held.release) })
		if err := peer.Close(); err != nil {
			t.Error(err)
		}
		if err := prefix.Close(); err != nil {
			t.Error(err)
		}
	})
	opened := make(chan error, 1)
	helpers.Go(func() { _, err := ReadClosedLaneFrame(peer); opened <- err })
	lane, err := prefix.channels.open(context.Background(), sourceIssuerOpen(end), end)
	if err := errors.Join(err, <-opened); err != nil {
		t.Fatal(err)
	}
	if err := child.activate(); err != nil {
		t.Fatal(err)
	}
	held.armed.Store(true)
	closed := make(chan error, 1)
	helpers.Go(func() { closed <- lane.Close() })
	select {
	case <-held.entered:
	case <-time.After(time.Second):
		t.Fatal("CLOSE did not reach its pre-emission boundary")
	}
	encoded, err := EncodeClosedLaneFrame(ClosedLaneFrame{Kind: closedFrameBytes, Lane: lane.id, Body: []byte("x")})
	if err != nil {
		t.Fatal(err)
	}
	credited := make(chan error, 1)
	helpers.Go(func() {
		if partial {
			var first [1]byte
			_, err := io.ReadFull(peer, first[:])
			credited <- err
			return
		}
		for range 2 {
			frame, err := ReadClosedLaneFrame(peer)
			if err != nil {
				credited <- err
				return
			}
			if frame.Kind != closedFrameCredit {
				credited <- errors.New("expected real CREDIT")
				return
			}
		}
		credited <- nil
	})
	if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: encoded}); err != nil {
		t.Fatal(err)
	}
	if err := <-credited; err != nil {
		t.Fatal(err)
	}
	// Peer consumption alone does not join the writer's completion.
	for !partial {
		child.mu.Lock()
		active, failed, changed := child.physicalWriting, child.physicalWriteFailed, child.writeChanged
		child.mu.Unlock()
		if failed {
			t.Fatal("CREDIT did not finish successfully")
		}
		if !active {
			break
		}
		select {
		case <-changed:
		case <-time.After(time.Second):
			t.Fatal("CREDIT writer did not finish")
		}
	}
	if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameClose, Lane: 1, Body: []byte{0}}); err != nil {
		t.Fatal(err)
	}
	for {
		child.mu.Lock()
		terminal, changed := child.terminal, child.writeChanged
		child.mu.Unlock()
		if terminal != nil {
			if terminal != io.EOF {
				t.Fatal(terminal)
			}
			break
		}
		select {
		case <-changed:
		case <-time.After(time.Second):
			t.Fatal("peer terminal not observed")
		}
	}
	unblock.Do(func() { close(held.release) })
	outcome := <-closed
	if partial {
		if !errors.Is(outcome, ErrClosedSourceCleanup) {
			t.Fatalf("failed CREDIT disappeared from cleanup: %v", outcome)
		}
		child.mu.Lock()
		failed := child.physicalWriteFailed
		child.mu.Unlock()
		if !failed {
			t.Fatal("partial CREDIT did not retain its physical failure")
		}
	} else if outcome != nil {
		t.Fatalf("successful unrelated CREDIT invented a CLOSE cleanup failure: %v", outcome)
	}
}
