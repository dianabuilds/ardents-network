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

// closedForwardingBlockedCloseCarrier makes cancellation observable before it
// interrupts an in-flight Carrier read. It keeps the callback alive until the
// test permits physical closure.
type closedForwardingBlockedCloseCarrier struct {
	route.Carrier
	closeEntered chan struct{}
	allowClose   <-chan struct{}
	done         chan struct{}
	once         sync.Once
	err          error
}

func (carrier *closedForwardingBlockedCloseCarrier) Close() error {
	carrier.once.Do(func() {
		close(carrier.closeEntered)
		<-carrier.allowClose
		carrier.err = carrier.Carrier.Close()
		close(carrier.done)
	})
	<-carrier.done
	return carrier.err
}

func TestClosedForwardingSessionCanceledLateAcceptWaitsForCloseAndReturnsNoSession(t *testing.T) {
	local, peer := net.Pipe()
	allowClose := make(chan struct{})
	carrier := &closedForwardingBlockedCloseCarrier{
		Carrier: local, closeEntered: make(chan struct{}), allowClose: allowClose, done: make(chan struct{}),
	}
	var workers sync.WaitGroup
	var allowOnce sync.Once
	releaseClose := func() { allowOnce.Do(func() { close(allowClose) }) }
	key := route.ClosedCarrierKey{NetworkID: [32]byte{31}, ProfileDigest: [32]byte{32}, LocalNodeID: [32]byte{33}, PeerNodeID: [32]byte{34}, PeerKey: [32]byte{35}, CarrierProfile: route.ClosedCarrierTCP}
	pool, err := route.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := pool.AcquireContext(t.Context(), key, func() error { return nil }, func() (route.Carrier, error) { return carrier, nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		releaseClose()
		_ = local.Close()
		_ = peer.Close()
		_ = lease.Release()
		_ = pool.Close()
		workers.Wait()
	})
	accept := mustClosedForwardAccept(t)
	helloRead := make(chan error, 1)
	allowAccept := make(chan struct{})
	var acceptOnce sync.Once
	releaseAccept := func() { acceptOnce.Do(func() { close(allowAccept) }) }
	acceptWritten := make(chan error, 1)
	t.Cleanup(releaseAccept)
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, readErr := route.ReadClosedLaneFrame(peer)
		helloRead <- readErr
		if readErr != nil {
			return
		}
		<-allowAccept
		acceptWritten <- route.WriteClosedLaneFrame(peer, accept)
	}()
	sessions := newClosedForwardingSessions(&workers)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	type outcome struct {
		session *closedForwardingSession
		err     error
	}
	creator := make(chan outcome, 1)
	deadline := time.Now().Add(5 * time.Second)
	helloDeadline := time.Now().UTC().Truncate(time.Second).Add(time.Minute)
	workers.Add(1)
	go func() {
		defer workers.Done()
		session, acquireErr := sessions.acquire(ctx, key, lease, deadline, func() (route.ClosedHello, error) {
			return route.ClosedHello{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, RecipientNodeID: [32]byte{5}, RecipientDutyGeneration: 6, Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{7}, Deadline: helloDeadline}, nil
		})
		creator <- outcome{session, acquireErr}
	}()
	select {
	case readErr := <-helloRead:
		if readErr != nil {
			t.Fatalf("outer HELLO read = %v", readErr)
		}
	case <-time.After(time.Second):
		t.Fatal("outer HELLO was not observed")
	}
	cancel()
	select {
	case <-carrier.closeEntered:
	case <-time.After(time.Second):
		t.Fatal("creator cancellation did not start Carrier close")
	}
	releaseAccept()
	select {
	case writeErr := <-acceptWritten:
		if writeErr != nil {
			t.Fatalf("late ACCEPT write = %v", writeErr)
		}
	case <-time.After(time.Second):
		t.Fatal("late ACCEPT was not consumed")
	}
	select {
	case got := <-creator:
		t.Fatalf("creator returned before cancellation close joined: %+v", got)
	case <-time.After(100 * time.Millisecond):
	}
	releaseClose()
	select {
	case got := <-creator:
		if got.session != nil || !errors.Is(got.err, context.Canceled) {
			t.Fatalf("canceled late ACCEPT creator = session %p error %v", got.session, got.err)
		}
	case <-time.After(time.Second):
		t.Fatal("creator did not return after cancellation close joined")
	}
	sessions.mu.Lock()
	_, pending := sessions.pending[key]
	_, published := sessions.sessions[key]
	sessions.mu.Unlock()
	if pending || published {
		t.Fatal("canceled late ACCEPT published a session")
	}
}
