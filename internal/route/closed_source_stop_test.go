//go:build linux

package route

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestClosedSourceParentStopJoinsInterruptedChildClose(t *testing.T) {
	owner, peer, end := sourceChannelsFixture(t)
	opened := make(chan error, 1)
	go func() { _, err := ReadClosedLaneFrame(peer); opened <- err }()
	lane, err := owner.open(context.Background(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- lane.Close() }()
	// The peer deliberately stops reading: terminal CLOSE owns the real writer.
	waitSourceChannelState(t, owner, func() bool { return owner.active != nil && owner.active.frame.Kind == closedFrameClose })
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("joined parent shutdown reported child cleanup failure: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parent shutdown left child CLOSE running")
	}
}

func TestClosedSourceChildCloseFailureStillRetiresParent(t *testing.T) {
	owner, peer, end := sourceChannelsFixture(t)
	opened := make(chan error, 1)
	go func() { _, err := ReadClosedLaneFrame(peer); opened <- err }()
	lane, err := owner.open(context.Background(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- lane.Close() }()
	waitSourceChannelState(t, owner, func() bool { return owner.active != nil && owner.active.frame.Kind == closedFrameClose })
	if err := peer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-closed; !errors.Is(err, ErrClosedSourceCleanup) {
		t.Fatalf("lost genuine CLOSE failure: %v", err)
	}
	select {
	case <-owner.done:
	default:
		t.Fatal("failed child CLOSE did not join parent")
	}
}

func TestClosedSourceQueuedCloseJoinsAlreadyFailedParent(t *testing.T) {
	for _, failedRetirement := range []bool{false, true} {
		name := "retired"
		if failedRetirement {
			name = "retirement-failed"
		}
		t.Run(name, func(t *testing.T) { testClosedSourceQueuedClose(t, failedRetirement) })
	}
}

func testClosedSourceQueuedClose(t *testing.T, failedRetirement bool) {
	owner, peer, end := sourceChannelsFixture(t)
	fault := errors.New("physical retirement failure")
	if failedRetirement {
		retire := owner.retire
		owner.retire = func() error { return errors.Join(retire(), fault) }
	}
	opened := make(chan error, 1)
	go func() {
		for range 2 {
			if _, err := ReadClosedLaneFrame(peer); err != nil {
				opened <- err
				return
			}
		}
		opened <- nil
	}()
	first, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	second, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	writeDone := make(chan error, 1)
	go func() { _, err := first.Write([]byte("unfinished")); writeDone <- err }()
	var partial [1]byte
	if _, err := peer.Read(partial[:]); err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- second.Close() }()
	waitSourceChannelState(t, owner, func() bool { return len(owner.controls) == 1 && owner.controls[0].frame.Kind == closedFrameClose })
	if err := peer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-writeDone; err == nil {
		t.Fatal("lost actual partial write failure")
	}
	if err := <-closed; failedRetirement && !errors.Is(err, fault) || !failedRetirement && err != nil {
		t.Fatalf("unemitted child terminal retirement result: %v", err)
	}
	select {
	case <-owner.done:
	default:
		t.Fatal("child cleanup did not join failed physical parent")
	}
	owner.mu.Lock()
	cause := owner.terminal
	owner.mu.Unlock()
	if cause == nil {
		t.Fatal("original parent failure was erased")
	}
}
