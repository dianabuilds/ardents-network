//go:build linux

package route

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

// Use the joined client's single-lane retirement policy and real framed I/O.
// Hold an emitted CREDIT so local Close is definitely waiting when peer CLOSE
// arrives. The peer's terminal must remain visible even after local admission
// is closed; unrelated late frames must not replenish credit.
func TestClosedJoinedLocalCloseRetainsConcurrentPeerTerminal(t *testing.T) {
	for _, status := range []byte{0, 1} {
		t.Run(fmt.Sprintf("status-%d", status), func(t *testing.T) {
			checkClosedJoinedConcurrentTerminal(t, status)
		})
	}
}

func checkClosedJoinedConcurrentTerminal(t *testing.T, status byte) {
	owner, peer, end := sourceChannelsFixture(t)
	owner.mu.Lock()
	owner.retainClosedRead = true
	owner.mu.Unlock()
	var workers sync.WaitGroup
	t.Cleanup(func() { _ = peer.Close(); _ = owner.Close(); workers.Wait() })
	opened := make(chan error, 1)
	workers.Go(func() { _, err := ReadClosedLaneFrame(peer); opened <- err })
	lane, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
	if peerErr := <-opened; err != nil || peerErr != nil {
		t.Fatal(errors.Join(err, peerErr))
	}
	credited, closed := make(chan error, 1), make(chan error, 1)
	workers.Go(func() {
		credited <- lane.send(ClosedLaneFrame{Kind: closedFrameCredit, Lane: lane.id, Body: binary.BigEndian.AppendUint32(nil, 1)}, time.Time{})
	})
	var first [1]byte
	if _, err := io.ReadFull(peer, first[:]); err != nil {
		t.Fatal(err)
	}
	workers.Go(func() { closed <- lane.Close() })
	waitSourceChannelState(t, owner, func() bool { return lane.closed })
	if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameClose, Lane: lane.id, Body: []byte{status}}); err != nil {
		t.Fatal(err)
	}
	// Reading this second real frame requires processing the preceding CLOSE.
	// Its CREDIT would overflow a live lane and must still be ignored here.
	if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameCredit, Lane: lane.id, Body: binary.BigEndian.AppendUint32(nil, 1)}); err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	observed, cause, credit := lane.remoteClosed, lane.failure, lane.credit
	owner.mu.Unlock()
	if !observed || (status == 0 && cause != io.EOF) || (status != 0 && (cause == nil || cause == io.EOF)) || credit != 64<<10 {
		t.Fatalf("local close discarded peer terminal or accepted late credit: terminal=%v cause=%v credit=%d", observed, cause, credit)
	}
	if _, err := io.ReadFull(peer, make([]byte, closedLaneHeaderSize+4-1)); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-credited:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CREDIT did not finish")
	}
	// There must be no redundant outgoing CLOSE requiring the peer to read again.
	select {
	case err := <-closed:
		if (status == 0 && err != nil) || (status != 0 && (!errors.Is(err, ErrClosedSourceCleanup) || !errors.Is(err, cause))) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("redundant CLOSE blocked retirement")
	}
}

func TestClosedJoinedCloseRetainsRefusalDuringTerminalWrite(t *testing.T) {
	owner, peer, end := sourceChannelsFixture(t)
	owner.mu.Lock()
	owner.retainClosedRead = true
	owner.mu.Unlock()
	var workers sync.WaitGroup
	t.Cleanup(func() { _ = peer.Close(); _ = owner.Close(); workers.Wait() })
	opened := make(chan error, 1)
	workers.Go(func() { _, err := ReadClosedLaneFrame(peer); opened <- err })
	lane, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
	if peerErr := <-opened; err != nil || peerErr != nil {
		t.Fatal(errors.Join(err, peerErr))
	}
	closed := make(chan error, 1)
	workers.Go(func() { closed <- lane.Close() })
	var first [1]byte
	if _, err := io.ReadFull(peer, first[:]); err != nil {
		t.Fatal(err)
	}
	if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameClose, Lane: lane.id, Body: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameCredit, Lane: lane.id, Body: binary.BigEndian.AppendUint32(nil, 1)}); err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	cause := lane.failure
	owner.mu.Unlock()
	if cause == nil || cause == io.EOF {
		t.Fatal("peer refusal was not processed")
	}
	if _, err := io.ReadFull(peer, make([]byte, closedLaneHeaderSize)); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-closed:
		if !errors.Is(err, ErrClosedSourceCleanup) || !errors.Is(err, cause) {
			t.Fatalf("concurrent peer refusal lost: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("terminal write did not retire")
	}
}
