//go:build linux

package route

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func TestClosedSourceRefusedLaneStillConsumesReceivedBytes(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	owner := newClosedSourceChannelOwner(local, time.Now().Add(time.Minute), local.Close)
	owner.transferred = (32 << 20) - 20
	if err := owner.receive(ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: []byte{0, 0, 0, 1}}); err == nil {
		t.Fatal("unallocated lane accepted")
	}
	if owner.transferred != 32<<20 {
		t.Fatalf("semantic refusal refunded received bytes: %d", owner.transferred)
	}
}

func TestClosedSourceCloseCannotExceedAdmissionBudget(t *testing.T) {
	owner, peer, end := sourceChannelsFixture(t)
	received := make(chan ardp.Frame, 2)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			frame, err := ardp.ReadFrame(peer)
			if err != nil {
				return
			}
			received <- frame
		}
	}()
	t.Cleanup(func() { _ = peer.Close(); <-done })
	lane, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if frame := <-received; frame.Kind != ardp.KindOpen {
		t.Fatal("missing OPEN")
	}
	owner.mu.Lock()
	owner.transferred = 32 << 20 // Isolate the exhausted parent; this is not a traffic run.
	owner.mu.Unlock()
	if err := lane.Close(); err == nil {
		t.Error("terminal CLOSE exceeded class-2 allowance")
	}
	_ = owner.Close()
	<-done
	select {
	case frame := <-received:
		t.Errorf("over-budget frame reached peer: kind %d", frame.Kind)
	default:
	}
}

func TestClosedSourcePartialWriteDoesNotRefundByteAllowance(t *testing.T) {
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
	owner.mu.Lock()
	owner.transferred = (32 << 20) - 17
	owner.mu.Unlock()
	failed := make(chan error, 1)
	go func() {
		// Receive exactly the header, then close before its one-byte payload.
		_, err := io.ReadFull(peer, make([]byte, 16))
		_ = peer.Close()
		failed <- err
	}()
	if _, err := lane.Write([]byte{1}); err == nil {
		t.Fatal("partial frame write succeeded")
	}
	if err := <-failed; err != nil {
		t.Fatal(err)
	}
	<-owner.done
	owner.mu.Lock()
	used := owner.transferred
	owner.mu.Unlock()
	if used != 32<<20 {
		t.Fatalf("failed output refunded its reservation: %d", used)
	}
}
