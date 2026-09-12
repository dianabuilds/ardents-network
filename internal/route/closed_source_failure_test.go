//go:build linux

package route

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestClosedSourceRetainsTransportFailureForNextOpen(t *testing.T) {
	owner, peer, end := sourceChannelsFixture(t)
	// A truncated actual frame produces io.ErrUnexpectedEOF in the owned reader.
	if _, err := peer.Write([]byte("ARD")); err != nil {
		t.Fatal(err)
	}
	if err := peer.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-owner.done:
	case <-time.After(time.Second):
		t.Fatal("source transport did not retire")
	}
	_, err := owner.open(t.Context(), sourceIssuerOpen(end), time.Now().Add(time.Second))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("first failure was hidden by next OPEN: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = owner.open(t.Context(), sourceIssuerOpen(end), time.Now().Add(time.Second))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("cleanup replaced original failure: %v", err)
	}
}

func TestClosedSourceReadPreservesOriginalTransportFailure(t *testing.T) {
	owner, peer, end := sourceChannelsFixture(t)
	if err := peer.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	opened := make(chan error, 1)
	go func() {
		frame, err := ReadClosedLaneFrame(peer)
		if err == nil && (frame.Kind != closedFrameOpen || frame.Lane != 1) {
			err = errors.New("unexpected opening")
		}
		opened <- err
	}()
	lane, err := owner.open(t.Context(), sourceIssuerOpen(end), time.Now().Add(time.Second))
	peerErr := <-opened
	if err != nil || peerErr != nil {
		t.Fatal(errors.Join(err, peerErr))
	}
	// The existing reader sees a genuinely truncated frame, not a synthetic
	// terminal assignment; subsequent lane reads must retain that first cause.
	if _, err := peer.Write([]byte("ARD")); err != nil {
		t.Fatal(err)
	}
	if err := peer.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-owner.done:
	case <-time.After(time.Second):
		t.Fatal("transport failure did not join")
	}
	var raw [1]byte
	_, err = lane.Read(raw[:])
	if !errors.Is(err, io.ErrUnexpectedEOF) || !errors.Is(err, net.ErrClosed) {
		t.Fatalf("read erased original transport failure: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = lane.Read(raw[:])
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("cleanup erased original read failure: %v", err)
	}
}
