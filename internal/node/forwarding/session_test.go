package forwarding

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
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
	body, err := ardp.EncodeHello(ardp.Hello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest,
		ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeForwarding,
		ChannelNonce: [32]byte{8}, Deadline: receiver.Deadline})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Accept(ardp.Frame{Kind: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	local, peer := net.Pipe()
	var workers sync.WaitGroup
	defer func() { _ = local.Close(); _ = peer.Close(); workers.Wait() }()
	if err := peer.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	session := &session{carrier: local, children: make(map[uint32]*frameQueue), retired: make(map[uint32]struct{})}
	start := make(chan struct{})
	results := make(chan error, count)
	open := route.ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeIssuer, Deadline: receiver.Deadline}
	for range count {
		workers.Go(func() {
			<-start
			_, _, err := session.attach(t.Context(), open, route.ClosedChildOrdinary, nil, nil)
			results <- err
		})
	}
	close(start)
	for index := 0; index < count; index++ {
		frame, err := ardp.ReadFrame(peer)
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

func TestClosedForwardingQueuesFragmentedBytesAndTerminalWithinByteBudget(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	if err := local.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	opened := make(chan error, 1)
	go func() { _, err := ardp.ReadFrame(peer); opened <- err }()
	session := &session{carrier: local, children: make(map[uint32]*frameQueue), retired: make(map[uint32]struct{})}
	reserved := 0
	lane, reverse, err := session.attach(t.Context(), route.ClosedOpen{NextNodeID: [32]byte{1}, NextDutyGeneration: 1, Purpose: ardp.PurposeIssuer,
		Deadline: time.Now().UTC().Truncate(time.Second).Add(time.Second)}, route.ClosedChildIssuerBootstrap,
		func(ardp.Frame) error { reserved++; return nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	session.children[lane].maximum = 64*17 + 17
	for range 64 {
		if !session.deliverReverse(ardp.Frame{Kind: 6, Lane: lane, Body: []byte{1}}) {
			t.Fatal("ordinary queue unexpectedly full")
		}
	}
	if session.deliverReverse(ardp.Frame{Kind: 6, Lane: lane, Body: make([]byte, 16<<10)}) {
		t.Fatal("terminal slot enlarged data allowance")
	}
	if !session.deliverReverse(ardp.Frame{Kind: 9, Lane: lane, Body: []byte{0}}) {
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

func TestClosedForwardingTransportFailureIsNotPeerTerminal(t *testing.T) {
	reverse := newFrameQueue(32)
	reverse.close()
	session := &session{closed: true}
	if written, err := session.writeChildFrame(ardp.Frame{Kind: 9, Lane: 1, Body: []byte{5}}, time.Now().Add(time.Second), reverse); err == nil || written {
		t.Fatal("physical EOF was treated as successful child terminal")
	}
}

func TestClosedForwardingPartialWriteFailureSurvivesPeerTerminal(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	reverse := newFrameQueue(32)
	session := &session{carrier: local}
	observed := make(chan error, 1)
	go func() {
		var first [1]byte
		_, err := peer.Read(first[:])
		if err == nil {
			err = reverse.push(ardp.Frame{Kind: 9, Lane: 1, Body: []byte{0}}, nil)
		}
		_ = peer.Close()
		observed <- err
	}()
	written, err := session.writeChildFrame(ardp.Frame{Kind: 6, Lane: 1, Body: []byte("payload")}, time.Now().Add(time.Second), reverse)
	if peerErr := <-observed; peerErr != nil {
		t.Fatal(peerErr)
	}
	if !reverse.peerClosed() || written || err == nil {
		t.Fatal("received terminal hid a partially emitted frame failure")
	}
}
