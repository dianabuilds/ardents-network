//go:build linux

package client

import (
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// A completed Introduction delivery leaves a short payload write deadline.
// Receiving the next delivery must return framing credit within the admitted
// lane lifetime, without extending permission to write a later payload.
func TestClosedSourceLaneCreditDoesNotReuseCompletedPayloadDeadline(t *testing.T) {
	owner, peer, end := sourceChannelsFixture(t)
	if err := peer.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	opened := make(chan error, 1)
	go func() { _, err := ardp.ReadFrame(peer); opened <- err }()
	lane, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	if err := lane.activate(); err != nil {
		t.Fatal(err)
	}
	if err := lane.SetWriteDeadline(time.Now()); err != nil {
		t.Fatal(err)
	}
	credited := make(chan error, 1)
	go func() {
		if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindBytes, Lane: lane.id, Body: []byte{7}}); err != nil {
			credited <- err
			return
		}
		frame, err := ardp.ReadFrame(peer)
		if err == nil && (frame.Kind != ardp.KindCredit || frame.Lane != lane.id || binary.BigEndian.Uint32(frame.Body) != 1) {
			err = errors.New("consumed byte did not receive its exact credit")
		}
		credited <- err
	}()
	var body [1]byte
	if n, err := lane.Read(body[:]); n != 1 || err != nil || body[0] != 7 {
		t.Fatalf("read: %d, %v", n, err)
	}
	owner.mu.Lock()
	failure := lane.failure
	owner.mu.Unlock()
	if failure != nil {
		t.Fatalf("completed payload deadline poisoned receive CREDIT: %v", failure)
	}
	if err := <-credited; err != nil {
		t.Fatal(err)
	}
	if _, err := lane.Write([]byte("late payload")); err == nil {
		t.Fatal("credit extended payload write authority")
	} else {
		var timeout net.Error
		if !errors.As(err, &timeout) || !timeout.Timeout() {
			t.Fatalf("payload deadline lost: %v", err)
		}
	}
}

func TestClosedSourceLaneDeadlineInterruptsActiveCredit(t *testing.T) {
	owner, peer, end := sourceChannelsFixture(t)
	opened := make(chan error, 1)
	go func() { _, err := ardp.ReadFrame(peer); opened <- err }()
	lane, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	if err := lane.activate(); err != nil {
		t.Fatal(err)
	}
	read := make(chan error, 1)
	go func() {
		var body [1]byte
		n, err := lane.Read(body[:])
		if err == nil && (n != 1 || body[0] != 7) {
			err = errors.New("received byte lost during credit interruption")
		}
		read <- err
	}()
	if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindBytes, Lane: lane.id, Body: []byte{7}}); err != nil {
		t.Fatal(err)
	}
	waitSourceChannelState(t, owner, func() bool {
		return owner.active != nil && owner.active.lane == lane && owner.active.frame.Kind == ardp.KindCredit && owner.active.attempted
	})
	if err := lane.SetWriteDeadline(time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := <-read; err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	failure, attempted := lane.failure, lane.physicalWriteFailed
	owner.mu.Unlock()
	var timeout net.Error
	if !attempted || !errors.As(failure, &timeout) || !timeout.Timeout() {
		t.Fatalf("active CREDIT deadline lost: physical failure=%t, error=%v", attempted, failure)
	}
}
