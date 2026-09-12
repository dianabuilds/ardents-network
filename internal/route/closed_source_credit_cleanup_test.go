//go:build linux

package route

import (
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClosedSourceCloseLetsEmittedCreditFinishWithinCleanupBound(t *testing.T) {
	for _, stalled := range []bool{false, true} {
		name := "peer-drains"
		if stalled {
			name = "peer-stalls"
		}
		t.Run(name, func(t *testing.T) { testClosedSourceCreditCleanup(t, stalled) })
	}
}

func testClosedSourceCreditCleanup(t *testing.T, stalled bool) {
	owner, peer, end := sourceChannelsFixture(t)
	opened := make(chan error, 1)
	go func() { _, err := ReadClosedLaneFrame(peer); opened <- err }()
	lane, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	creditDone := make(chan error, 1)
	go func() {
		creditDone <- lane.send(ClosedLaneFrame{Kind: closedFrameCredit, Lane: lane.id, Body: binary.BigEndian.AppendUint32(nil, 1)}, time.Time{})
	}()
	// Hold the actual physical CREDIT after its first header byte. Closing this
	// child must not cut a valid frame in half and poison the retained prefix.
	var first [1]byte
	if _, err := io.ReadFull(peer, first[:]); err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- lane.Close() }()
	waitSourceChannelState(t, owner, func() bool { return lane.closed })
	select {
	case err := <-creditDone:
		t.Fatalf("cleanup interrupted an in-flight credit before its bound: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if stalled {
		// A concurrent TLS owner may reset a deadline while Close is joining.
		// That cannot replace the cleanup bound of the already active frame.
		if err := lane.SetWriteDeadline(end); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-closed:
			if err == nil {
				t.Fatal("stalled credit cleanup succeeded")
			}
		case <-time.After(1500 * time.Millisecond):
			t.Fatal("stalled credit exceeded cleanup bound")
		}
		if err := <-creditDone; err == nil {
			t.Fatal("partial physical credit succeeded")
		}
		select {
		case <-owner.done:
		case <-time.After(time.Second):
			t.Fatal("failed cleanup did not join prefix")
		}
		return
	}
	rest := make([]byte, closedLaneHeaderSize+4-1)
	if _, err := io.ReadFull(peer, rest); err != nil {
		t.Fatal(err)
	}
	if err := <-creditDone; err != nil {
		t.Fatal(err)
	}
	terminal, err := ReadClosedLaneFrame(peer)
	if err != nil || terminal.Kind != closedFrameClose {
		t.Fatalf("terminal cleanup: %v / %v", terminal, err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	go func() { _, err := ReadClosedLaneFrame(peer); opened <- err }()
	if _, err := owner.open(t.Context(), sourceIssuerOpen(end), end); err != nil {
		t.Fatalf("retained prefix unavailable: %v", err)
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
}

// The peer closes after reading one real CREDIT header byte while local Close
// is joining that active write. Preserve both the physical cause and its phase;
// calling this a failed CLOSE frame sends diagnosis down the wrong path.
func TestClosedSourceCloseIdentifiesInterruptedCredit(t *testing.T) {
	owner, peer, end := sourceChannelsFixture(t)
	var workers sync.WaitGroup
	t.Cleanup(func() { _ = peer.Close(); _ = owner.Close(); workers.Wait() })
	opened := make(chan error, 1)
	workers.Go(func() { _, err := ReadClosedLaneFrame(peer); opened <- err })
	lane, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
	if peerErr := <-opened; err != nil || peerErr != nil {
		t.Fatal(errors.Join(err, peerErr))
	}
	credited := make(chan error, 1)
	closed := make(chan error, 1)
	workers.Go(func() {
		credited <- lane.send(ClosedLaneFrame{Kind: closedFrameCredit, Lane: lane.id, Body: binary.BigEndian.AppendUint32(nil, 1)}, time.Time{})
	})
	var first [1]byte
	if _, err := io.ReadFull(peer, first[:]); err != nil {
		t.Fatal(err)
	}
	workers.Go(func() { closed <- lane.Close() })
	waitSourceChannelState(t, owner, func() bool { return lane.closed })
	if err := peer.Close(); err != nil {
		t.Fatal(err)
	}
	var creditErr, closeErr error
	select {
	case creditErr = <-credited:
	case <-time.After(2 * time.Second):
		t.Fatal("CREDIT did not join")
	}
	select {
	case closeErr = <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not join")
	}
	if creditErr == nil || !errors.Is(closeErr, creditErr) || !errors.Is(closeErr, ErrClosedSourceCleanup) {
		t.Fatalf("lost interrupted CREDIT cause: credit=%v cleanup=%v", creditErr, closeErr)
	}
	if !strings.Contains(closeErr.Error(), "in-flight CREDIT write failed") || strings.Contains(closeErr.Error(), "CLOSE write failed") {
		t.Fatalf("wrong cleanup phase: %v", closeErr)
	}
}
