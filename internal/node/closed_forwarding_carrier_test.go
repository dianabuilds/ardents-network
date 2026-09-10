package node

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestClosedForwardingSessionSharesOneOuterHelloAndDemultiplexesChildren(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	key := route.ClosedCarrierKey{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, LocalNodeID: [32]byte{3}, PeerNodeID: [32]byte{4}, PeerKey: [32]byte{5}, CarrierProfile: route.ClosedCarrierTCP}
	deadline := time.Now().UTC().Truncate(time.Second).Add(time.Minute)
	server := make(chan struct{})
	go func() {
		defer close(server)
		frame, err := route.ReadClosedLaneFrame(peer)
		if err != nil || frame.Kind != 1 || frame.Lane != 0 {
			return
		}
		if err := route.WriteClosedLaneFrame(peer, mustClosedForwardAccept(t)); err != nil {
			return
		}
		for lane := uint32(1); lane <= 3; lane += 2 {
			frame, err = route.ReadClosedLaneFrame(peer)
			if err != nil || frame.Kind != 4 || frame.Lane != lane {
				return
			}
			if err := route.WriteClosedLaneFrame(peer, route.ClosedLaneFrame{Kind: 6, Lane: lane, Body: []byte{byte(lane)}}); err != nil {
				return
			}
		}
	}()
	sessions := newClosedForwardingSessions(&sync.WaitGroup{})
	hello := func() (route.ClosedHello, error) {
		return route.ClosedHello{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4},
			RecipientNodeID: [32]byte{5}, RecipientDutyGeneration: 6, Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{7}, Deadline: deadline}, nil
	}
	pool, err := route.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	binding, err := pool.Acquire(key, func() error { return nil }, func() (route.Carrier, error) { return local, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer binding.Release()
	session, err := sessions.acquire(key, binding, time.Now().Add(10*time.Second), hello)
	if err != nil {
		t.Fatal(err)
	}
	open := route.ClosedOpen{NextNodeID: [32]byte{5}, NextDutyGeneration: 6, Purpose: route.ClosedPurposeForwarding, Deadline: deadline}
	first, firstReverse, err := session.attach(open, route.ClosedChildOrdinary, nil, nil)
	if err != nil || first != 1 {
		t.Fatalf("first child = %d / %v", first, err)
	}
	second, secondReverse, err := session.attach(open, route.ClosedChildOrdinary, nil, nil)
	if err != nil || second != 3 {
		t.Fatalf("second child = %d / %v", second, err)
	}
	for lane, reverse := range map[uint32]*closedForwardingQueue{first: firstReverse, second: secondReverse} {
		received := make(chan route.ClosedLaneFrame, 1)
		go func() { frame, _ := reverse.next(); received <- frame }()
		select {
		case frame := <-received:
			if frame.Kind != 6 || frame.Lane != lane || len(frame.Body) != 1 || frame.Body[0] != byte(lane) {
				t.Fatalf("reverse lane %d = %+v", lane, frame)
			}
		case <-time.After(time.Second):
			t.Fatalf("reverse lane %d was not delivered", lane)
		}
	}
	session.fail()
	<-server
}

func mustClosedForwardAccept(t *testing.T) route.ClosedLaneFrame {
	t.Helper()
	frame, err := route.ClosedAcceptFrame(0, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func TestClosedForwardingRetirementRacesReverseDeliveryWithoutClosedChannelSend(t *testing.T) {
	for range 1000 {
		frames := newClosedForwardingQueue(68)
		session := &closedForwardingSession{children: map[uint32]*closedForwardingQueue{1: frames}, retired: make(map[uint32]struct{})}
		start, delivered, retired := make(chan struct{}), make(chan bool, 1), make(chan struct{})
		go func() {
			<-start
			delivered <- session.deliverReverse(route.ClosedLaneFrame{Kind: 6, Lane: 1, Body: []byte{1}})
		}()
		go func() { <-start; session.retire(1); close(retired) }()
		close(start)
		if !<-delivered {
			t.Fatal("live or retired lane was refused")
		}
		<-retired
		for {
			if _, ok := frames.next(); !ok {
				break
			}
		}
		if !session.deliverReverse(route.ClosedLaneFrame{Kind: 9, Lane: 1, Body: []byte{0}}) {
			t.Fatal("retired terminal was refused")
		}
		if session.deliverReverse(route.ClosedLaneFrame{Kind: 6, Lane: 1, Body: []byte{1}}) {
			t.Fatal("terminated lane resurrected")
		}
	}
}

func TestClosedForwardingQueueExhaustionTerminatesCarrierAndAllChildren(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	if err := peer.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	first, second := newClosedForwardingQueue(68), newClosedForwardingQueue(68)
	invalidated := make(chan struct{})
	session := &closedForwardingSession{owner: newClosedForwardingSessions(&sync.WaitGroup{}), carrier: local,
		invalidate: func() error { close(invalidated); return nil },
		children:   map[uint32]*closedForwardingQueue{1: first, 3: second}, retired: make(map[uint32]struct{})}
	done := make(chan struct{})
	go func() { session.copyReverse(); close(done) }()
	for range 4 + 1 {
		if err := route.WriteClosedLaneFrame(peer, route.ClosedLaneFrame{Kind: 6, Lane: 1, Body: []byte{1}}); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("full child queue blocked Carrier termination")
	}
	select {
	case <-invalidated:
	default:
		t.Fatal("failed Carrier remained reusable")
	}
	count := 0
	for {
		if _, ok := first.next(); !ok {
			break
		}
		count++
	}
	if count != 4 {
		t.Fatalf("queued frames = %d", count)
	}
	if _, open := second.next(); open {
		t.Fatal("other child survived failed Carrier")
	}
	if session.deliverReverse(route.ClosedLaneFrame{Kind: 6, Lane: 3, Body: []byte{1}}) {
		t.Fatal("failed Carrier accepted late output")
	}
}
