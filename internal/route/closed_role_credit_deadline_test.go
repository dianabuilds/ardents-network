//go:build linux

package route

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// The retained framing layer's CREDIT belongs to its parent reservation. A
// completed inner payload write's deadline must not poison later consumption.
func TestClosedRoleChildCreditDoesNotReuseCompletedPayloadDeadline(t *testing.T) {
	local, peer := net.Pipe()
	end := time.Now().Add(3 * time.Second)
	if err := peer.SetDeadline(end); err != nil {
		t.Fatal(err)
	}
	stream := newClosedRoleChildStream(local, end, local.Close, nil)
	var workers sync.WaitGroup
	t.Cleanup(func() { _ = peer.Close(); _ = stream.Close(); workers.Wait() })
	if err := stream.activate(); err != nil {
		t.Fatal(err)
	}
	initial := make(chan error, 1)
	workers.Go(func() { _, err := ReadClosedLaneFrame(peer); initial <- err })
	if _, err := stream.Write([]byte("completed payload")); err != nil {
		t.Fatal(err)
	}
	if err := <-initial; err != nil {
		t.Fatal(err)
	}
	// Reproduce the deadline left by a completed child cleanup, without sleeping.
	if err := stream.SetWriteDeadline(time.Now()); err != nil {
		t.Fatal(err)
	}
	received := make(chan error, 1)
	workers.Go(func() {
		if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{7}}); err != nil {
			received <- err
			return
		}
		credit, err := ReadClosedLaneFrame(peer)
		if err == nil && (credit.Kind != closedFrameCredit || binary.BigEndian.Uint32(credit.Body) != 1) {
			err = io.ErrUnexpectedEOF
		}
		received <- err
	})
	var body [1]byte
	if n, err := stream.Read(body[:]); n != 1 || err != nil || body[0] != 7 {
		t.Fatalf("read: %d %v", n, err)
	}
	stream.mu.Lock()
	terminal := stream.terminal
	stream.mu.Unlock()
	if terminal != nil {
		t.Fatalf("completed payload deadline poisoned receive CREDIT: %v", terminal)
	}
	if err := <-received; err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Write([]byte("late payload")); err == nil {
		t.Fatal("credit processing extended payload write authority")
	} else {
		var timeout net.Error
		if !errors.As(err, &timeout) || !timeout.Timeout() {
			t.Fatalf("payload deadline lost: %v", err)
		}
	}
}

func TestClosedRoleChildDeadlineInterruptsEmittedCredit(t *testing.T) {
	local, peer := net.Pipe()
	end := time.Now().Add(3 * time.Second)
	if err := peer.SetDeadline(end); err != nil {
		t.Fatal(err)
	}
	stream := newClosedRoleChildStream(local, end, local.Close, nil)
	var workers sync.WaitGroup
	t.Cleanup(func() { _ = peer.Close(); _ = stream.Close(); workers.Wait() })
	if err := stream.activate(); err != nil {
		t.Fatal(err)
	}
	read := make(chan error, 1)
	workers.Go(func() {
		var body [1]byte
		n, err := stream.Read(body[:])
		if err == nil && (n != 1 || body[0] != 7) {
			err = io.ErrUnexpectedEOF
		}
		read <- err
	})
	if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{7}}); err != nil {
		t.Fatal(err)
	}
	var prefix [1]byte
	if _, err := io.ReadFull(peer, prefix[:]); err != nil {
		t.Fatal(err)
	}
	if err := stream.SetWriteDeadline(time.Now()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-read:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("deadline failed to interrupt emitted CREDIT")
	}
	stream.mu.Lock()
	terminal := stream.terminal
	stream.mu.Unlock()
	var timeout net.Error
	if !errors.As(terminal, &timeout) || !timeout.Timeout() {
		t.Fatalf("physical CREDIT failure was lost: %v", terminal)
	}
}

func TestClosedRoleChildPayloadDeadlineBoundsCreditWriterWait(t *testing.T) {
	local, peer := net.Pipe()
	end := time.Now().Add(3 * time.Second)
	if err := peer.SetDeadline(end); err != nil {
		t.Fatal(err)
	}
	stream := newClosedRoleChildStream(local, end, local.Close, nil)
	var workers sync.WaitGroup
	t.Cleanup(func() { _ = peer.Close(); _ = stream.Close(); workers.Wait() })
	if err := stream.activate(); err != nil {
		t.Fatal(err)
	}
	if err := stream.SetWriteDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	workers.Go(func() { var body [1]byte; _, _ = stream.Read(body[:]) })
	if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{7}}); err != nil {
		t.Fatal(err)
	}
	var prefix [1]byte
	if _, err := io.ReadFull(peer, prefix[:]); err != nil {
		t.Fatal(err)
	}
	written := make(chan error, 1)
	workers.Go(func() { _, err := stream.Write([]byte("bounded payload")); written <- err })
	select {
	case err := <-written:
		var timeout net.Error
		if !errors.As(err, &timeout) || !timeout.Timeout() {
			t.Fatalf("payload deadline lost: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("payload waited beyond its deadline behind CREDIT")
	}
}
