package node

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestClosedForwardingConcurrentAttachPreservesWireIDOrder(t *testing.T) {
	const count = 32
	now := time.Now().UTC().Truncate(time.Second)
	limits, err := route.NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	receiver := route.ClosedOuterReceiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4},
		NodeID: [32]byte{5}, RecordDigest: [32]byte{6}, DutyGeneration: 7, RoleDomain: 2, Subrole: 6, Deadline: now.Add(10 * time.Second)}
	outer, err := route.NewClosedOuterHandshake(receiver, limits, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer outer.Close()
	body, err := route.EncodeClosedHello(route.ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest,
		ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: route.ClosedPurposeForwarding,
		ChannelNonce: [32]byte{8}, Deadline: receiver.Deadline})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Accept(route.ClosedLaneFrame{Kind: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	local, peer := net.Pipe()
	var workers sync.WaitGroup
	defer func() { _ = local.Close(); _ = peer.Close(); workers.Wait() }()
	if err := peer.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	session := &closedForwardingSession{carrier: local, children: make(map[uint32]*closedForwardingQueue), retired: make(map[uint32]struct{})}
	start := make(chan struct{})
	results := make(chan error, count)
	open := route.ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration, Purpose: route.ClosedPurposeIssuer, Deadline: receiver.Deadline}
	for range count {
		workers.Go(func() {
			<-start
			_, _, err := session.attach(open, route.ClosedChildOrdinary, nil, nil)
			results <- err
		})
	}
	close(start)
	for index := 0; index < count; index++ {
		frame, err := route.ReadClosedLaneFrame(peer)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := outer.Accept(frame); err != nil {
			t.Fatalf("concurrent sender emitted nonmonotonic OPEN %d at index%d: %v", frame.Lane, index, err)
		}
	}
	workers.Wait()
	for range count {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
}
