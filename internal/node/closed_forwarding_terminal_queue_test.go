package node

import (
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestClosedForwardingQueuesFragmentedBytesAndTerminalWithinByteBudget(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	if err := local.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	opened := make(chan error, 1)
	go func() { _, err := route.ReadClosedLaneFrame(peer); opened <- err }()
	session := &closedForwardingSession{carrier: local, children: make(map[uint32]*closedForwardingQueue), retired: make(map[uint32]struct{})}
	reserved := 0
	lane, reverse, err := session.attach(route.ClosedOpen{NextNodeID: [32]byte{1}, NextDutyGeneration: 1, Purpose: route.ClosedPurposeIssuer,
		Deadline: time.Now().UTC().Truncate(time.Second).Add(time.Second)}, route.ClosedChildIssuerBootstrap,
		func(route.ClosedLaneFrame) error { reserved++; return nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	session.children[lane].maximum = 64*17 + 17
	for range 64 {
		if !session.deliverReverse(route.ClosedLaneFrame{Kind: 6, Lane: lane, Body: []byte{1}}) {
			t.Fatal("ordinary queue unexpectedly full")
		}
	}
	if session.deliverReverse(route.ClosedLaneFrame{Kind: 6, Lane: lane, Body: make([]byte, 16<<10)}) {
		t.Fatal("terminal slot enlarged data allowance")
	}
	if !session.deliverReverse(route.ClosedLaneFrame{Kind: 9, Lane: lane, Body: []byte{0}}) {
		t.Fatal("full data queue discarded terminal CLOSE")
	}
	if reserved != 65 {
		t.Fatalf("unreserved terminal or reserved rejected data: %d", reserved)
	}
	for index := 0; index < 65; index++ {
		frame, _ := reverse.next()
		if index < 64 && frame.Kind != 6 || index == 64 && frame.Kind != 9 {
			t.Fatal("terminal overtook queued bytes")
		}
	}
}
