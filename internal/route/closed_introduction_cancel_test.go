//go:build linux

package route

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestClosedIntroductionWithdrawalCancelInterruptsWrite(t *testing.T) {
	parent, peer, end := sourceChannelsFixture(t)
	opened := make(chan error, 1)
	go func() { _, err := ReadClosedLaneFrame(peer); opened <- err }()
	lane, err := parent.open(context.Background(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	// Exercise the registration lifecycle with real framing and a blocked mux
	// writer. TLS and actual recipient admission are covered by network tests.
	owner := &ClosedIntroductionRegistration{lane: lane, connection: lane, writer: make(chan struct{}, 1),
		request: ClosedRegistrationRequest{Slot: [32]byte{1}, Revision: 1, Expiry: end},
		stop:    func() bool { return true }, done: make(chan struct{})}
	go owner.read()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- owner.Withdraw(ctx) }()
	waitSourceChannelState(t, parent, func() bool { return parent.active != nil && parent.active.frame.Kind == closedFrameBytes })
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("withdrawal lost cancellation: %v", err)
		}
	case <-time.After(2 * time.Second):
		_ = parent.Close()
		<-finished
		t.Fatal("caller cancellation did not interrupt withdrawal write")
	}
	select {
	case <-owner.Done():
	default:
		t.Fatal("withdrawal left registration reader alive")
	}
}
