//go:build linux

package node

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// Exercise the production openForwardingLink caller while its successful
// producer remains live across Stop. The server must join that producer before
// the session owner begins its final reader wait.
func TestClosedForwardingDrainJoinsLateActualOpenProducerBeforeReader(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	fixture := newClosedBootstrapFixture(t)
	fixture.now = now
	fixture.view.Profile.NotBefore = now.Add(-time.Second)
	fixture.view.Profile.NotAfter = now.Add(time.Minute)
	fixture.snapshot.EpochValidFrom = fixture.view.Profile.NotBefore
	fixture.snapshot.ValidUntil = fixture.view.Profile.NotAfter
	fixture.snapshot.RecordValidFrom = fixture.view.Profile.NotBefore
	fixture.snapshot.RecordValidUntil = fixture.view.Profile.NotAfter
	for index := range fixture.snapshot.Candidates {
		fixture.snapshot.Candidates[index].ValidFrom = fixture.view.Profile.NotBefore
		fixture.snapshot.Candidates[index].ValidUntil = fixture.view.Profile.NotAfter
		fixture.snapshot.Candidates[index].AssignmentNotAfter = fixture.view.Profile.NotAfter
	}
	fixture.config.now = time.Now
	fixture.config.Current = func() (DutyView, error) { return fixture.snapshot, nil }
	fixture.config.CurrentClosedProfile = func() (state.ClosedProfileView, bool) { return fixture.view.Profile, true }
	open := fixture.open
	open.Deadline = now.Add(5 * time.Second)
	candidate, err := closedForwardRecipient(fixture.config, fixture.snapshot, open, now)
	if err != nil {
		t.Fatal(err)
	}
	receiver, available := closedRouteReceiver(fixture.config, fixture.snapshot, route.ClosedPurposeForwarding, now)
	if !available {
		t.Fatal("forwarding receiver unavailable")
	}
	key := route.ClosedCarrierKey{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, LocalNodeID: receiver.NodeID,
		PeerNodeID: candidate.NodeID, PeerKey: candidate.PublicKey, CarrierProfile: route.CarrierProfile(candidate.CarrierProfile)}
	pool, err := route.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	binding := route.ClosedSpendBinding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest,
		ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration}
	spends, err := route.OpenClosedSpendLedger(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	server := newClosedForwardingServerWithHost(fixture.config, fixture.snapshot, tls.Certificate{}, idleForwardingListener{}, spends, nil, pool, nil, nil, 1)
	local, peer := net.Pipe()
	closeErr := errors.New("fixture physical close failed")
	blocked := &delayedForwardingRead{Conn: local, gate: make(chan struct{}), interrupted: make(chan struct{}), closeErr: closeErr}
	seed, err := pool.AcquireContext(t.Context(), key, func() error { return nil }, func() (route.Carrier, error) { return blocked, nil })
	if err != nil {
		t.Fatal(err)
	}
	allowAccept := make(chan struct{})
	allowProducer := make(chan struct{})
	allowPeerExit := make(chan struct{})
	var releaseAccept, releaseProducer, releaseReader, releasePeer sync.Once
	t.Cleanup(func() {
		releaseAccept.Do(func() { close(allowAccept) })
		releaseProducer.Do(func() { close(allowProducer) })
		releaseReader.Do(func() { close(blocked.gate) })
		releasePeer.Do(func() { close(allowPeerExit) })
		_ = peer.Close()
		_ = seed.Release()
		_ = server.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Drain(ctx)
		_ = spends.Close()
	})
	helloRead := make(chan error, 1)
	childRead := make(chan error, 1)
	peerDone := make(chan struct{})
	go func() {
		defer close(peerDone)
		frame, readErr := route.ReadClosedLaneFrame(peer)
		if readErr == nil && (frame.Kind != 1 || frame.Lane != 0) {
			readErr = errors.New("outer HELLO was altered")
		}
		helloRead <- readErr
		if readErr != nil {
			return
		}
		<-allowAccept
		accept := mustClosedForwardAccept(t)
		if writeErr := route.WriteClosedLaneFrame(peer, accept); writeErr != nil {
			childRead <- writeErr
			return
		}
		frame, readErr = route.ReadClosedLaneFrame(peer)
		if readErr == nil && (frame.Kind != 4 || frame.Lane != 1) {
			readErr = errors.New("child OPEN was altered")
		}
		childRead <- readErr
		<-allowPeerExit
	}()
	type openResult struct {
		link *closedForwardingLink
		err  error
	}
	producerResult := make(chan openResult, 1)
	producerDone := make(chan struct{})
	channel := closedForwardingActualChannel(t, open.Deadline, open)
	server.workers.Add(1)
	go func() {
		defer server.workers.Done()
		link, openErr := server.openForwardingLink(context.Background(), open, route.ClosedChildOrdinary, 1, channel, func(route.ClosedLaneFrame) error { return nil }, func() {})
		producerResult <- openResult{link: link, err: openErr}
		<-allowProducer
		close(producerDone)
	}()
	if err := <-helloRead; err != nil {
		t.Fatal(err)
	}
	releaseAccept.Do(func() { close(allowAccept) })
	if err := <-childRead; err != nil {
		t.Fatal(err)
	}
	if err := server.Stop(); err != nil {
		t.Fatal(err)
	}
	opened := <-producerResult
	if opened.err != nil || opened.link == nil {
		t.Fatalf("late actual open = link %p error %v", opened.link, opened.err)
	}
	first, cancelFirst := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancelFirst()
	if err := server.Drain(first); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Drain completed before producer join: %v", err)
	}
	if replacement, err := route.OpenClosedSpendLedger(root, binding); err == nil {
		_ = replacement.Close()
		t.Fatal("live producer lost its spend root")
	}
	releaseProducer.Do(func() { close(allowProducer) })
	select {
	case <-producerDone:
	case <-time.After(time.Second):
		t.Fatal("late actual open producer did not join")
	}
	readerOnly, cancelReaderOnly := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancelReaderOnly()
	if err := server.Drain(readerOnly); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Drain completed before delayed reader join: %v", err)
	}
	if replacement, err := route.OpenClosedSpendLedger(root, binding); err == nil {
		_ = replacement.Close()
		t.Fatal("delayed reader lost its spend root")
	}
	select {
	case <-blocked.interrupted:
	case <-time.After(time.Second):
		t.Fatal("Stop did not interrupt the outgoing Carrier")
	}
	releaseReader.Do(func() { close(blocked.gate) })
	releasePeer.Do(func() { close(allowPeerExit) })
	<-peerDone
	joined, cancelJoined := context.WithTimeout(t.Context(), time.Second)
	defer cancelJoined()
	for range 2 {
		if err := server.Drain(joined); !errors.Is(err, closeErr) {
			t.Fatalf("joined cleanup changed its physical close result: %v", err)
		}
	}
	if err := opened.link.close(); !errors.Is(err, closeErr) && err != nil {
		t.Fatal(err)
	}
	reopened, err := route.OpenClosedSpendLedger(root, binding)
	if err != nil {
		t.Fatalf("joined shutdown retained root: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}
