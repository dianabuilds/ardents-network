package node

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

type closedForwardingObservedContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func (context *closedForwardingObservedContext) Done() <-chan struct{} {
	context.once.Do(func() { close(context.observed) })
	return context.Context.Done()
}

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
	sessions := newClosedForwardingSessions()
	hello := func() (route.ClosedHello, error) {
		return route.ClosedHello{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4},
			RecipientNodeID: [32]byte{5}, RecipientDutyGeneration: 6, Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{7}, Deadline: deadline}, nil
	}
	pool, err := route.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	binding, err := pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (route.Carrier, error) { return local, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer binding.Release()
	session, err := sessions.acquire(context.Background(), key, binding, time.Now().Add(10*time.Second), hello)
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
	if err := sessions.joinedResult(); err != nil {
		t.Fatal(err)
	}
}

func TestClosedForwardingSessionsReadyCarrierProgressesWhileOtherHelloBlocks(t *testing.T) {
	firstLocal, firstPeer := net.Pipe()
	secondLocal, secondPeer := net.Pipe()
	accept := mustClosedForwardAccept(t)
	var workers sync.WaitGroup
	var sessions *closedForwardingSessions
	var releaseOnce sync.Once
	releaseFirst := make(chan struct{})
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(releaseFirst) })
		_ = firstLocal.Close()
		_ = firstPeer.Close()
		_ = secondLocal.Close()
		_ = secondPeer.Close()
		workers.Wait()
		if sessions != nil {
			_ = sessions.joinedResult()
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	helloDeadline := time.Now().UTC().Truncate(time.Second).Add(time.Minute)
	firstKey := route.ClosedCarrierKey{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, LocalNodeID: [32]byte{3}, PeerNodeID: [32]byte{4}, PeerKey: [32]byte{5}, CarrierProfile: route.ClosedCarrierTCP}
	secondKey := firstKey
	secondKey.PeerNodeID[0] = 6
	pool, err := route.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	firstLease, err := pool.AcquireContext(t.Context(), firstKey, func() error { return nil }, func() (route.Carrier, error) { return firstLocal, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer firstLease.Release()
	secondLease, err := pool.AcquireContext(t.Context(), secondKey, func() error { return nil }, func() (route.Carrier, error) { return secondLocal, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer secondLease.Release()
	firstHelloRead := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		if _, readErr := route.ReadClosedLaneFrame(firstPeer); readErr != nil {
			firstHelloRead <- readErr
			return
		}
		firstHelloRead <- nil
		<-releaseFirst
		_ = route.WriteClosedLaneFrame(firstPeer, accept)
	}()
	workers.Add(1)
	go func() {
		defer workers.Done()
		if _, readErr := route.ReadClosedLaneFrame(secondPeer); readErr == nil {
			_ = route.WriteClosedLaneFrame(secondPeer, accept)
		}
	}()
	hello := func() (route.ClosedHello, error) {
		return route.ClosedHello{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, RecipientNodeID: [32]byte{5}, RecipientDutyGeneration: 6, Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{7}, Deadline: helloDeadline}, nil
	}
	sessions = newClosedForwardingSessions()
	firstResult := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, acquireErr := sessions.acquire(context.Background(), firstKey, firstLease, deadline, hello)
		firstResult <- acquireErr
	}()
	select {
	case readErr := <-firstHelloRead:
		if readErr != nil {
			t.Fatalf("first outer HELLO read = %v", readErr)
		}
	case acquireErr := <-firstResult:
		t.Fatalf("first outer HELLO returned before peer read: %v", acquireErr)
	case <-time.After(time.Second):
		t.Fatal("first outer HELLO was not observed")
	}
	waiterBase, cancelWaiter := context.WithCancel(t.Context())
	defer cancelWaiter()
	waiterContext := &closedForwardingObservedContext{Context: waiterBase, observed: make(chan struct{})}
	waiterResult := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, acquireErr := sessions.acquire(waiterContext, firstKey, firstLease, deadline, hello)
		waiterResult <- acquireErr
	}()
	select {
	case <-waiterContext.observed:
	case <-time.After(time.Second):
		t.Fatal("same-key waiter did not enter cancellation wait")
	}
	cancelWaiter()
	select {
	case acquireErr := <-waiterResult:
		if !errors.Is(acquireErr, context.Canceled) {
			t.Fatalf("canceled same-key waiter = %v", acquireErr)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled same-key waiter remained blocked")
	}
	select {
	case acquireErr := <-firstResult:
		t.Fatalf("canceled waiter stopped owner: %v", acquireErr)
	default:
	}
	secondResult := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, acquireErr := sessions.acquire(context.Background(), secondKey, secondLease, deadline, hello)
		secondResult <- acquireErr
	}()
	select {
	case acquireErr := <-secondResult:
		if acquireErr != nil {
			t.Fatalf("ready second Carrier = %v", acquireErr)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked first HELLO delayed ready second Carrier")
	}
	releaseOnce.Do(func() { close(releaseFirst) })
	select {
	case acquireErr := <-firstResult:
		if acquireErr != nil {
			t.Fatalf("released first owner = %v", acquireErr)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked first HELLO did not join")
	}
}

func TestClosedForwardingSessionsReuseReadyCarrierWhileOtherHelloBlocks(t *testing.T) {
	firstLocal, firstPeer := net.Pipe()
	secondLocal, secondPeer := net.Pipe()
	var workers sync.WaitGroup
	var sessions *closedForwardingSessions
	t.Cleanup(func() {
		_ = firstLocal.Close()
		_ = firstPeer.Close()
		_ = secondLocal.Close()
		_ = secondPeer.Close()
		workers.Wait()
		if sessions != nil {
			_ = sessions.joinedResult()
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	helloDeadline := time.Now().UTC().Truncate(time.Second).Add(time.Minute)
	firstKey := route.ClosedCarrierKey{NetworkID: [32]byte{11}, ProfileDigest: [32]byte{12}, LocalNodeID: [32]byte{13}, PeerNodeID: [32]byte{14}, PeerKey: [32]byte{15}, CarrierProfile: route.ClosedCarrierTCP}
	secondKey := firstKey
	secondKey.PeerNodeID[0] = 16
	pool, err := route.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	firstLease, err := pool.AcquireContext(t.Context(), firstKey, func() error { return nil }, func() (route.Carrier, error) { return firstLocal, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer firstLease.Release()
	secondLease, err := pool.AcquireContext(t.Context(), secondKey, func() error { return nil }, func() (route.Carrier, error) { return secondLocal, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer secondLease.Release()
	hello := func() (route.ClosedHello, error) {
		return route.ClosedHello{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, RecipientNodeID: [32]byte{5}, RecipientDutyGeneration: 6, Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{7}, Deadline: helloDeadline}, nil
	}
	accept := mustClosedForwardAccept(t)
	secondReady := make(chan struct{})
	workers.Add(1)
	go func() {
		defer workers.Done()
		frame, readErr := route.ReadClosedLaneFrame(secondPeer)
		if readErr != nil || frame.Kind != 1 {
			return
		}
		if route.WriteClosedLaneFrame(secondPeer, accept) != nil {
			return
		}
		close(secondReady)
		frame, readErr = route.ReadClosedLaneFrame(secondPeer)
		if readErr == nil && frame.Kind == 4 {
			_ = route.WriteClosedLaneFrame(secondPeer, route.ClosedLaneFrame{Kind: 6, Lane: frame.Lane, Body: []byte{1}})
		}
	}()
	sessions = newClosedForwardingSessions()
	ready, err := sessions.acquire(t.Context(), secondKey, secondLease, deadline, hello)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-secondReady:
	case <-time.After(time.Second):
		t.Fatal("second Carrier did not become ready")
	}
	firstSeen := make(chan struct{})
	workers.Add(1)
	go func() {
		defer workers.Done()
		if _, readErr := route.ReadClosedLaneFrame(firstPeer); readErr == nil {
			close(firstSeen)
		}
	}()
	firstResult := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, acquireErr := sessions.acquire(t.Context(), firstKey, firstLease, deadline, hello)
		firstResult <- acquireErr
	}()
	select {
	case <-firstSeen:
	case <-time.After(time.Second):
		t.Fatal("first HELLO did not block")
	}
	reused, err := sessions.acquire(t.Context(), secondKey, secondLease, deadline, hello)
	if err != nil || reused != ready {
		t.Fatalf("ready Carrier reuse = %p / %p / %v", reused, ready, err)
	}
	open := route.ClosedOpen{NextNodeID: [32]byte{5}, NextDutyGeneration: 6, Purpose: route.ClosedPurposeForwarding, Deadline: helloDeadline}
	lane, reverse, err := reused.attach(open, route.ClosedChildOrdinary, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	frame, ok := reverse.next()
	if !ok || lane == 0 || frame.Kind != 6 || frame.Lane != lane {
		t.Fatalf("ready Carrier child progress = lane %d frame %+v open %v", lane, frame, ok)
	}
	select {
	case acquireErr := <-firstResult:
		t.Fatalf("ready B stopped blocked A: %v", acquireErr)
	default:
	}
}

func TestClosedForwardingSessionCreatorCancellationInterruptsBlockedHello(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	key := route.ClosedCarrierKey{NetworkID: [32]byte{21}, ProfileDigest: [32]byte{22}, LocalNodeID: [32]byte{23}, PeerNodeID: [32]byte{24}, PeerKey: [32]byte{25}, CarrierProfile: route.ClosedCarrierTCP}
	pool, err := route.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	lease, err := pool.AcquireContext(t.Context(), key, func() error { return nil }, func() (route.Carrier, error) { return local, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	seen := make(chan error, 1)
	go func() { _, readErr := route.ReadClosedLaneFrame(peer); seen <- readErr }()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sessions := newClosedForwardingSessions()
	result := make(chan error, 1)
	deadline := time.Now().Add(5 * time.Second)
	helloDeadline := time.Now().UTC().Truncate(time.Second).Add(time.Minute)
	go func() {
		_, acquireErr := sessions.acquire(ctx, key, lease, deadline, func() (route.ClosedHello, error) {
			return route.ClosedHello{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, RecipientNodeID: [32]byte{5}, RecipientDutyGeneration: 6, Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{7}, Deadline: helloDeadline}, nil
		})
		result <- acquireErr
	}()
	if readErr := <-seen; readErr != nil {
		t.Fatal(readErr)
	}
	cancel()
	select {
	case acquireErr := <-result:
		if !errors.Is(acquireErr, context.Canceled) {
			t.Fatalf("creator cancel = %v", acquireErr)
		}
	case <-time.After(time.Second):
		t.Fatal("creator cancellation did not interrupt HELLO")
	}
	sessions.mu.Lock()
	_, pending := sessions.pending[key]
	_, published := sessions.sessions[key]
	sessions.mu.Unlock()
	if pending || published {
		t.Fatal("canceled creator published a session")
	}
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

func TestClosedForwardingRetirementDoesNotRetainTombstoneAfterPeerClose(t *testing.T) {
	session := &closedForwardingSession{children: make(map[uint32]*closedForwardingQueue), retired: make(map[uint32]struct{})}
	for index := uint32(0); index < 512; index++ {
		lane := index*2 + 1
		frames := newClosedForwardingQueue(68)
		session.children[lane] = frames
		if !session.deliverReverse(route.ClosedLaneFrame{Kind: 9, Lane: lane, Body: []byte{0}}) {
			t.Fatalf("peer CLOSE %d was refused", index)
		}
		session.retire(lane)
	}
	if len(session.children) != 0 || len(session.retired) != 0 {
		t.Fatalf("terminal children retained: children=%d retired=%d", len(session.children), len(session.retired))
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
	session := &closedForwardingSession{owner: newClosedForwardingSessions(), carrier: local,
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
