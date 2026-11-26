package channel

import (
	"context"
	"net"
	"testing"
	"time"
)

// Actual pipe framing checks the terminal/witness boundary, not successful
// Network, Admission, Service authentication or an accepting JOIN.
func TestRefusedCloseCannotProduceCleanPeerWitness(t *testing.T) {
	local, remote := net.Pipe()
	end := time.Now().Add(time.Minute)
	sender, sendLane, err := newJoinedSession(context.Background(), local, end, 1<<20, nil, NewBudget(4<<20))
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	receiver, receiveLane, err := newJoinedSession(context.Background(), remote, end, 1<<20, nil, NewBudget(4<<20))
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()
	result := make(chan error, 1)
	go func() { result <- sendLane.CloseRefused() }()
	select {
	case <-receiver.Done():
	case <-time.After(time.Second):
		t.Fatal("peer did not decode the original refusal")
	}
	if _, err := receiveLane.DecodedPeerClose(); err == nil {
		t.Fatal("failed retirement supplied a clean-CLOSE witness")
	}
	if err := <-result; err != nil {
		t.Fatal("original refusal writer did not join", err)
	}
	_ = receiver.Close() // Retains the peer refusal as work, not physical failure.
	if receiver.PhysicalFailure() != nil {
		t.Fatal("refusal fabricated a physical cleanup failure", receiver.PhysicalFailure())
	}
}
