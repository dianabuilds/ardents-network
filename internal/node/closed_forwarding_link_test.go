package node

import (
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestClosedForwardingLinkPreservesHalfCloseCreditsOnceAndJoinsReverse(t *testing.T) {
	for _, timing := range []string{"delivered-terminal", "queued-terminal", "blocked-terminal"} {
		t.Run(timing, func(t *testing.T) { testClosedForwardingLinkCompletion(t, timing) })
	}
}

func testClosedForwardingLinkCompletion(t *testing.T, timing string) {
	clock := time.Now
	deadline := time.Now().UTC().Truncate(time.Second).Add(8 * time.Second)
	governor, _ := route.NewClosedBootstrapController(clock)
	limits, _ := route.NewClosedDutyLimits(clock)
	reservation, err := governor.Admit([32]byte{1}, deadline)
	if err != nil {
		t.Fatal(err)
	}
	channel, err := route.NewClosedBootstrapForwardingChannel(reservation, limits, func(route.ClosedOpen) error { return nil }, clock)
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Cancel()
	body, _ := route.EncodeClosedOpen(route.ClosedOpen{NextNodeID: [32]byte{3}, NextDutyGeneration: 4, Purpose: route.ClosedPurposeIssuer, Deadline: deadline})
	if _, err := channel.Accept(route.ClosedLaneFrame{Kind: 4, Lane: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	channel.Next() // The already selected next hop is the explicit pipe fixture below.
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	if err := local.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	if err := peer.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	pool, _ := route.NewClosedCarrierPool(clock)
	defer pool.Close()
	key := route.ClosedCarrierKey{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, LocalNodeID: [32]byte{3},
		PeerNodeID: [32]byte{4}, PeerKey: [32]byte{5}, CarrierProfile: route.ClosedCarrierTCP}
	lease, err := pool.Acquire(key, func() error { return nil }, func() (route.Carrier, error) { return local, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.MarkUsed(); err != nil {
		t.Fatal(err)
	}
	// Four complete fixture frames: CREDIT, response BYTES, EOF and CLOSE.
	reverse := newClosedForwardingQueue(4*16 + 4 + len("response") + 1)
	session := &closedForwardingSession{owner: newClosedForwardingSessions(&sync.WaitGroup{}), carrier: local, invalidate: func() error { return nil },
		children: map[uint32]*closedForwardingQueue{3: reverse}, retired: make(map[uint32]struct{}),
		queues: map[uint32]func(route.ClosedLaneFrame) error{3: func(frame route.ClosedLaneFrame) error { frame.Lane = 1; return channel.QueueReverse(frame) }}}
	published := make(chan route.ClosedLaneFrame, 8)
	aborted := make(chan struct{}, 1)
	closeWriting, allowClose := make(chan struct{}), make(chan struct{})
	var allowOnce sync.Once
	releaseClose := func() { allowOnce.Do(func() { close(allowClose) }) }
	defer releaseClose()
	link := &closedForwardingLink{session: session, remoteLane: 3, localLane: 1, reverse: reverse, lease: lease, channel: channel,
		done: make(chan struct{}), stopped: make(chan struct{}), abort: func() { aborted <- struct{}{} },
		write: func(frame route.ClosedLaneFrame) error {
			if err := channel.AccountOutput(frame); err != nil {
				return err
			}
			if timing == "blocked-terminal" && frame.Kind == 9 {
				close(closeWriting)
				<-allowClose
			}
			published <- frame
			return nil
		}}
	sessionDone, peerDone := make(chan struct{}), make(chan error, 1)
	go func() { session.copyReverse(); close(sessionDone) }()
	// Hold the child writer until the shared reader has queued the complete
	// response. Normal completion must not depend on goroutine scheduling.
	reverseMayRun := make(chan struct{})
	var releaseOnce sync.Once
	releaseReverse := func() { releaseOnce.Do(func() { close(reverseMayRun) }) }
	go func() { <-reverseMayRun; link.copyReverse() }()
	defer func() {
		_ = local.Close()
		_ = peer.Close()
		releaseReverse()
		releaseClose()
		link.stop()
		<-link.done
		<-sessionDone
	}()
	go func() {
		defer peer.Close()
		frame, err := route.ReadClosedLaneFrame(peer)
		if err != nil {
			peerDone <- err
			return
		}
		if frame.Kind != 6 || frame.Lane != 3 || string(frame.Body) != "request" {
			peerDone <- net.ErrClosed
			return
		}
		frame, err = route.ReadClosedLaneFrame(peer)
		if err != nil {
			peerDone <- err
			return
		}
		if frame.Kind != 8 || frame.Lane != 3 {
			peerDone <- net.ErrClosed
			return
		}
		for _, frame := range []route.ClosedLaneFrame{
			{Kind: 7, Lane: 3, Body: binary.BigEndian.AppendUint32(nil, uint32(len("request")))},
			{Kind: 6, Lane: 3, Body: []byte("response")}, {Kind: 8, Lane: 3}, {Kind: 9, Lane: 3, Body: []byte{0}},
		} {
			if err := route.WriteClosedLaneFrame(peer, frame); err != nil {
				peerDone <- err
				return
			}
		}
		peerDone <- nil
	}()
	for _, frame := range []route.ClosedLaneFrame{{Kind: 6, Lane: 1, Body: []byte("request")}, {Kind: 8, Lane: 1}} {
		if _, err := channel.Accept(frame); err != nil {
			t.Fatal(err)
		}
		event, available := channel.Next()
		if !available || event.Kind != frame.Kind {
			t.Fatal("outbound event missing")
		}
		if written, err := session.writeChildFrame(route.ClosedLaneFrame{Kind: event.Kind, Lane: 3, Body: event.Bytes}, deadline, reverse); err != nil || !written {
			t.Fatalf("outbound frame was not emitted: written=%v err=%v", written, err)
		}
	}
	if err := <-peerDone; err != nil {
		t.Fatal(err)
	}
	select {
	case <-aborted:
		t.Fatal("normal directional completion aborted prefix")
	default:
	}
	<-sessionDone
	if timing == "queued-terminal" {
		// The complete peer CLOSE and physical EOF are already observed, but
		// the child copier has not run. Local cleanup cannot depend on it.
		if _, err := channel.Accept(route.ClosedLaneFrame{Kind: 9, Lane: 1, Body: []byte{5}}); err != nil {
			t.Fatal(err)
		}
		links := map[uint32]*closedForwardingLink{1: link}
		finished := make(chan error, 1)
		go func() {
			finished <- (&closedForwardingServer{}).drainForwarding(t.Context(), channel, links, link.write, link.abort)
		}()
		select {
		case err := <-finished:
			t.Fatalf("cleanup returned before joining held child copier: %v", err)
		case <-time.After(50 * time.Millisecond):
		}
		releaseReverse()
		select {
		case err := <-finished:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("cleanup did not join released copier")
		}
		if len(links) != 0 {
			t.Fatal("completed child remained allocated")
		}
		select {
		case <-aborted:
			t.Fatal("queued peer terminal aborted retained prefix")
		default:
		}
		return
	}
	releaseReverse()
	if timing == "blocked-terminal" {
		select {
		case <-closeWriting:
		case <-time.After(time.Second):
			t.Fatal("terminal writer not reached")
		}
		if _, err := channel.Accept(route.ClosedLaneFrame{Kind: 7, Lane: 1, Body: binary.BigEndian.AppendUint32(nil, uint32(len("response")))}); err != nil {
			t.Fatal(err)
		}
		finished := make(chan error, 1)
		go func() {
			finished <- (&closedForwardingServer{}).drainForwarding(t.Context(), channel, map[uint32]*closedForwardingLink{1: link}, link.write, link.abort)
		}()
		select {
		case err := <-finished:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("held terminal writer blocked queued upstream credit")
		}
		releaseClose()
	}
	for _, kind := range []uint8{7, 6, 8, 9} {
		select {
		case frame := <-published:
			if frame.Kind != kind || frame.Lane != 1 {
				t.Fatalf("published kind/lane = %d/%d", frame.Kind, frame.Lane)
			}
			if kind == 6 && string(frame.Body) != "response" {
				t.Fatal("response altered")
			}
		case <-time.After(time.Second):
			t.Fatal("reverse completion missing")
		}
	}
	select {
	case <-published:
		t.Fatal("duplicate reverse credit or completion")
	default:
	}
	if err := link.close(); err != nil {
		t.Fatal(err)
	}
	// The upstream owner releases its local child after receiving remote CLOSE.
	// The next carrier has already ended, so cleanup must not write there again.
	if _, err := channel.Accept(route.ClosedLaneFrame{Kind: 9, Lane: 1, Body: []byte{5}}); err != nil {
		t.Fatal(err)
	}
	links := map[uint32]*closedForwardingLink{1: link}
	server := &closedForwardingServer{}
	if err := server.drainForwarding(t.Context(), channel, links, link.write, link.abort); err != nil {
		t.Fatalf("completed child cleanup poisoned retained prefix: %v", err)
	}
	if len(links) != 0 {
		t.Fatal("completed child remained allocated")
	}
}

func TestClosedForwardingRetiredReverseCannotAbortSiblingOrSharedCarrier(t *testing.T) {
	clock := time.Now
	deadline := time.Now().UTC().Truncate(time.Second).Add(8 * time.Second)
	governor, _ := route.NewClosedBootstrapController(clock)
	limits, _ := route.NewClosedDutyLimits(clock)
	reservation, err := governor.Admit([32]byte{1}, deadline)
	if err != nil {
		t.Fatal(err)
	}
	channel, err := route.NewClosedBootstrapForwardingChannel(reservation, limits, func(route.ClosedOpen) error { return nil }, clock)
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Cancel()
	for _, lane := range []uint32{1, 3} {
		body, _ := route.EncodeClosedOpen(route.ClosedOpen{NextNodeID: [32]byte{2}, NextDutyGeneration: 3, Purpose: route.ClosedPurposeIssuer, Deadline: deadline})
		if _, err := channel.Accept(route.ClosedLaneFrame{Kind: 4, Lane: lane, Body: body}); err != nil {
			t.Fatal(err)
		}
		channel.Next()
	}
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	pool, _ := route.NewClosedCarrierPool(clock)
	defer pool.Close()
	key := route.ClosedCarrierKey{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, LocalNodeID: [32]byte{3}, PeerNodeID: [32]byte{4}, PeerKey: [32]byte{5}, CarrierProfile: route.ClosedCarrierTCP}
	lease, err := pool.Acquire(key, func() error { return nil }, func() (route.Carrier, error) { return local, nil })
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := pool.Acquire(key, func() error { return nil }, func() (route.Carrier, error) { t.Fatal("redialed live pair"); return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer sibling.Release()
	first, second := newClosedForwardingQueue(80), newClosedForwardingQueue(80)
	session := &closedForwardingSession{children: map[uint32]*closedForwardingQueue{3: first, 5: second}, retired: make(map[uint32]struct{}),
		queues: map[uint32]func(route.ClosedLaneFrame) error{
			3: func(frame route.ClosedLaneFrame) error { frame.Lane = 1; return channel.QueueReverse(frame) },
			5: func(frame route.ClosedLaneFrame) error { frame.Lane = 3; return channel.QueueReverse(frame) },
		}}
	session.retirements = map[uint32]func() bool{3: func() bool { return channel.ReverseRetired(1) }, 5: func() bool { return channel.ReverseRetired(3) }}
	late := route.ClosedLaneFrame{Kind: 6, Lane: 3, Body: []byte("late")}
	for range 4 {
		if !session.deliverReverse(late) {
			t.Fatal("live reverse refused")
		}
	}
	if _, err := channel.Accept(route.ClosedLaneFrame{Kind: 9, Lane: 1, Body: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	channel.Next()
	// Reproduce both windows: the reader sees retirement before session.stop,
	// and one already queued output completes after the same retirement.
	if !session.deliverReverse(late) || len(first.frames) != 4 {
		t.Fatal("late reader failed shared Carrier or queued retired output")
	}
	aborted, written := false, false
	link := &closedForwardingLink{session: session, remoteLane: 3, localLane: 1, reverse: first, lease: lease, channel: channel,
		done: make(chan struct{}), stopped: make(chan struct{}), abort: func() { aborted = true },
		write: func(frame route.ClosedLaneFrame) error {
			if err := channel.AccountOutput(frame); err != nil {
				return err
			}
			written = true
			return nil
		}}
	go link.copyReverse()
	if err := link.close(); err != nil {
		t.Fatal(err)
	}
	if aborted || written {
		t.Fatal("retired output escaped or aborted sibling prefix")
	}
	if _, err := sibling.Carrier(); err != nil {
		t.Fatal("closed one child invalidated shared Carrier")
	}
	if !session.deliverReverse(route.ClosedLaneFrame{Kind: 6, Lane: 5, Body: []byte("sibling")}) {
		t.Fatal("sibling reverse refused")
	}
	frame, _ := second.next()
	frame.Lane = 3
	if err := channel.AccountOutput(frame); err != nil {
		t.Fatal(err)
	}
	channel.ReleaseReverse(3, uint64(16+len(frame.Body)))
	session.retire(5)
}
