//go:build linux

package route

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

type joinedCloseWriteBoundary struct {
	net.Conn
	entered, release chan struct{}
	once             sync.Once
	partial          bool
}

func (connection *joinedCloseWriteBoundary) Write(value []byte) (int, error) {
	count := 0
	var err error
	connection.once.Do(func() {
		if connection.partial && len(value) > 0 {
			count, err = connection.Conn.Write(value[:1])
		}
		close(connection.entered)
		<-connection.release
	})
	if err != nil {
		return count, err
	}
	written, err := connection.Conn.Write(value[count:])
	return count + written, err
}

// The local inner CLOSE reaches its transport only after the actual outer
// peer CLOSE has been accepted. It must not turn a wholly unemitted cleanup
// frame into a shared-prefix failure.
func TestClosedJoinedUnemittedCloseAfterOuterPeerClose(t *testing.T) {
	for _, mode := range []string{"clean", "credit", "failed-credit", "refused", "partial"} {
		t.Run(mode, func(t *testing.T) { checkJoinedCloseBoundary(t, mode) })
	}
}

func checkJoinedCloseBoundary(t *testing.T, mode string) {
	outer, peer, end := sourceChannelsFixture(t)
	var tasks sync.WaitGroup
	t.Cleanup(tasks.Wait)
	creditRelease := make(chan struct{})
	creditPrefix := make(chan struct{})
	var releaseCredit sync.Once
	if mode != "credit" && mode != "failed-credit" {
		releaseCredit.Do(func() { close(creditRelease) })
	}
	peerDone := make(chan struct{})
	go func() {
		defer close(peerDone)
		for index := 0; ; index++ {
			if index == 1 {
				<-creditRelease
				if mode == "failed-credit" {
					var first [1]byte
					if _, err := io.ReadFull(peer, first[:]); err == nil {
						close(creditPrefix)
					}
					return
				}
			}
			if _, err := ReadClosedLaneFrame(peer); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		releaseCredit.Do(func() { close(creditRelease) })
		if err := peer.Close(); err != nil {
			t.Error(err)
		}
		<-peerDone
	})
	lane, err := outer.open(t.Context(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if err := lane.activate(); err != nil {
		t.Fatal(err)
	}
	creditDone := make(chan error, 1)
	if mode == "credit" || mode == "failed-credit" {
		if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameBytes, Lane: lane.id, Body: []byte{1}}); err != nil {
			t.Fatal(err)
		}
		tasks.Go(func() { var value [1]byte; _, err := io.ReadFull(lane, value[:]); creditDone <- err })
		waitSourceChannelState(t, outer, func() bool { return outer.active != nil && outer.active.frame.Kind == closedFrameCredit })
	}
	heldRead := &joinedReadBoundary{Conn: lane, entered: make(chan struct{}), release: make(chan struct{})}
	heldWrite := &joinedCloseWriteBoundary{Conn: heldRead, partial: mode == "partial", entered: make(chan struct{}), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(t.Context())
	released := make(chan struct{})
	stream := newClosedJoinedStream(ctx, heldWrite, lane, func() { close(released) })
	var unblockRead, unblockWrite sync.Once
	t.Cleanup(func() {
		releaseCredit.Do(func() { close(creditRelease) })
		unblockRead.Do(func() { close(heldRead.release) })
		unblockWrite.Do(func() { close(heldWrite.release) })
		cancel()
		closeErr := stream.Close()
		if (mode == "clean" || mode == "credit") && closeErr != nil {
			t.Errorf("clean cleanup: %v", closeErr)
		}
		if (mode != "clean" && mode != "credit") && !errors.Is(closeErr, ErrClosedSourceCleanup) {
			t.Errorf("%s cleanup lost failure: %v", mode, closeErr)
		}
		select {
		case <-released:
		default:
			t.Error("cleanup returned before reservation release")
		}
		tasks.Wait()
	})
	select {
	case <-heldRead.entered:
	case <-time.After(time.Second):
		t.Fatal("inner read did not reach boundary")
	}
	done := make(chan error, 1)
	tasks.Go(func() { done <- stream.Close() })
	select {
	case <-heldWrite.entered:
	case <-time.After(time.Second):
		t.Fatal("inner CLOSE did not reach transport")
	}
	if mode == "credit" || mode == "failed-credit" {
		releaseCredit.Do(func() { close(creditRelease) })
		if mode == "credit" {
			if err := <-creditDone; err != nil {
				t.Fatal(err)
			}
		} else {
			select {
			case <-creditPrefix:
			case <-time.After(time.Second):
				t.Fatal("CREDIT did not emit its prefix")
			}
		}
	}
	status := byte(0)
	if mode == "refused" {
		status = 1
	}
	if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameClose, Lane: lane.id, Body: []byte{status}}); err != nil {
		t.Fatal(err)
	}
	waitSourceChannelState(t, outer, func() bool { return lane.remoteClosed })
	if mode == "failed-credit" {
		if err := peer.Close(); err != nil {
			t.Fatal(err)
		}
		<-creditDone
		waitSourceChannelState(t, outer, func() bool { return outer.active == nil && lane.physicalWriteFailed })
	}
	unblockWrite.Do(func() { close(heldWrite.release) })
	waitSourceChannelState(t, stream.channels, func() bool { return stream.channels.terminal != nil })
	unblockRead.Do(func() { close(heldRead.release) })
	select {
	case err := <-done:
		if (mode == "clean" || mode == "credit") && err != nil {
			t.Fatalf("unemitted inner CLOSE poisoned cleanup: %v", err)
		}
		if (mode != "clean" && mode != "credit") && !errors.Is(err, ErrClosedSourceCleanup) {
			t.Fatalf("%s cleanup lost failure: %v", mode, err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("inner CLOSE did not join")
	}
	select {
	case <-released:
	default:
		t.Fatal("returned before reservation release")
	}
	outer.mu.Lock()
	failure := outer.terminal
	outer.mu.Unlock()
	if (mode == "clean" || mode == "credit") && failure != nil {
		t.Fatalf("clean outer closure poisoned retained prefix: %v", failure)
	}
}
