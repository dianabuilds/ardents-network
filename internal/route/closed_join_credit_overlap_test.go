//go:build linux

package route

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

type joinedCreditWitnessBoundary struct {
	net.Conn
	afterWrite func(error)
}

func (boundary *joinedCreditWitnessBoundary) Write(value []byte) (int, error) {
	written, err := boundary.Conn.Write(value)
	boundary.afterWrite(err)
	return written, err
}

// A previously emitted outer CREDIT completes while the inner CREDIT waits.
// The latter emits nothing after actual outer CLOSE and must preserve the tail.
func TestClosedJoinedCreditOverlapsCompletedOuterCredit(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "completed"
		if failed {
			name = "failed"
		}
		t.Run(name, func(t *testing.T) { checkClosedJoinedOuterCreditOverlap(t, failed) })
	}
}
func checkClosedJoinedOuterCreditOverlap(t *testing.T, failed bool) {
	outer, peer, end := sourceChannelsFixture(t)
	releasePeer := make(chan struct{})
	var peerOnce sync.Once
	var workers sync.WaitGroup
	workers.Go(func() {
		for index := 0; ; index++ {
			if index == 3 {
				<-releasePeer
				if failed {
					_ = peer.Close()
					return
				}
			}
			if _, err := ReadClosedLaneFrame(peer); err != nil {
				return
			}
		}
	})
	t.Cleanup(func() {
		peerOnce.Do(func() { close(releasePeer) })
		_ = peer.Close()
		_ = outer.Close()
		workers.Wait()
	})
	lane, err := outer.open(t.Context(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if err := lane.activate(); err != nil {
		t.Fatal(err)
	}
	heldRead := &joinedReadBoundary{Conn: lane, remaining: closedLaneHeaderSize + 1, entered: make(chan struct{}), release: make(chan struct{})}
	heldWrite := &joinedCloseWriteBoundary{Conn: heldRead, entered: make(chan struct{}), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(t.Context())
	stream := newClosedJoinedStream(ctx, heldWrite, lane, func() {})
	var readOnce, writeOnce sync.Once
	t.Cleanup(func() {
		readOnce.Do(func() { close(heldRead.release) })
		writeOnce.Do(func() { close(heldWrite.release) })
		cancel()
		_ = stream.Close()
	})
	var encoded []byte
	for _, frame := range []ClosedLaneFrame{{Kind: 6, Lane: 1, Body: []byte("a")}, {Kind: 6, Lane: 1, Body: []byte("tail")}, {Kind: 9, Lane: 1, Body: []byte{0}}} {
		raw, e := EncodeClosedLaneFrame(frame)
		if e != nil {
			t.Fatal(e)
		}
		encoded = append(encoded, raw...)
	}
	if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: 6, Lane: lane.id, Body: encoded}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-heldRead.entered:
	case <-time.After(time.Second):
		t.Fatal("reader boundary")
	}
	outerCredit := make(chan error, 1)
	workers.Go(func() {
		outerCredit <- lane.send(ClosedLaneFrame{Kind: 7, Lane: lane.id, Body: binary.BigEndian.AppendUint32(nil, 1)}, time.Time{})
	})
	waitSourceChannelState(t, outer, func() bool { return outer.active != nil && outer.active.frame.Kind == 7 })
	firstRead := make(chan error, 1)
	workers.Go(func() { var b [1]byte; _, e := io.ReadFull(stream, b[:]); firstRead <- e })
	select {
	case <-heldWrite.entered:
	case <-time.After(time.Second):
		t.Fatal("inner credit boundary")
	}
	if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: 9, Lane: lane.id, Body: []byte{0}}); err != nil {
		t.Fatal(err)
	}
	waitSourceChannelState(t, outer, func() bool { return lane.remoteClosed })
	peerOnce.Do(func() { close(releasePeer) })
	if err := <-outerCredit; (err != nil) != failed {
		t.Fatal(err)
	}
	writeOnce.Do(func() { close(heldWrite.release) })
	if err := <-firstRead; err != nil {
		t.Fatal(err)
	}
	readOnce.Do(func() { close(heldRead.release) })
	tail, err := io.ReadAll(stream)
	if failed {
		if err == nil {
			t.Fatal("failed outer CREDIT became success")
		}
		return
	}
	if err != nil || string(tail) != "tail" {
		t.Fatalf("completed outer CREDIT discarded tail: %q: %v", tail, err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClosedJoinedCreditWitnessDistinguishesPeerEOFAndLocalClose(t *testing.T) {
	for _, localFirst := range []bool{false, true} {
		name := "peer EOF before concurrent local close"
		if localFirst {
			name = "local close before write"
		}
		t.Run(name, func(t *testing.T) {
			end := time.Now().Add(time.Minute)
			lower := newClosedSourceChannelOwner(nil, end, func() error { return nil })
			parent := &closedSourceLane{owner: lower, id: 1, end: end, readEnd: end, writeEnd: end,
				opened: true, active: true, remoteClosed: true, failure: io.EOF}
			lower.lanes[1] = parent
			if localFirst {
				parent.closed = true
			}
			boundary := &joinedCreditWitnessBoundary{Conn: parent, afterWrite: func(err error) {
				if errors.Is(err, io.EOF) {
					lower.mu.Lock()
					parent.closed = true
					lower.mu.Unlock()
				}
			}}
			owner := newClosedSourceChannelOwner(boundary, end, func() error { return nil })
			owner.retainClosedRead = true
			owner.framedParent = parent
			lane := &closedSourceLane{owner: owner, id: 1, end: end, readEnd: end, writeEnd: end, opened: true, active: true}
			owner.lanes[1] = lane
			owner.workers.Add(1)
			go owner.write()
			err := lane.send(ClosedLaneFrame{Kind: closedFrameCredit, Lane: 1, Body: binary.BigEndian.AppendUint32(nil, 1)}, time.Time{})
			owner.mu.Lock()
			if owner.terminal == nil {
				owner.terminal = ErrClosedSourceStopped
				owner.signalLocked()
			}
			owner.mu.Unlock()
			owner.workers.Wait()
			if localFirst {
				if !errors.Is(err, net.ErrClosed) {
					t.Fatalf("local close = %v, want net.ErrClosed", err)
				}
			} else if err != nil {
				t.Fatalf("authenticated peer retirement became CREDIT failure: %v", err)
			}
		})
	}
}
