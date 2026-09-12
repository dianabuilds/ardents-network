package node

import (
	"context"
	"github.com/dianabuilds/ardents-network/internal/route"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestClosedOuterWriterDeadlineInterruptsRetainedCarrier(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	end := now.Add(time.Hour)
	receiver := route.ClosedOuterReceiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4},
		NodeID: [32]byte{5}, RecordDigest: [32]byte{6}, DutyGeneration: 7, RoleDomain: 2, Subrole: 6, Deadline: end}
	limits, err := route.NewClosedDutyLimits(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	outer, err := route.NewClosedOuterHandshake(receiver, limits, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	if err := peer.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	written := make(chan error, 1)
	go func() {
		defer close(done)
		serveClosedOuter(t.Context(), local, outer, func(_ context.Context, lane *route.ClosedOuterBridgeLane) {
			_, err := lane.Write([]byte{1})
			written <- err
		})
	}()
	hello := route.ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest,
		RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{8}, Deadline: end}
	body, err := route.EncodeClosedHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if err := route.WriteClosedLaneFrame(peer, route.ClosedLaneFrame{Kind: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	if _, err := route.ReadClosedLaneFrame(peer); err != nil {
		t.Fatal(err)
	}
	body, err = route.EncodeClosedNodeOpen(route.ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration,
		Purpose: route.ClosedPurposeIssuer, Deadline: now.Add(2 * time.Second)}, route.ClosedChildIssuerBootstrap)
	if err != nil {
		t.Fatal(err)
	}
	if err := route.WriteClosedLaneFrame(peer, route.ClosedLaneFrame{Kind: 4, Lane: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	// Peer deliberately never reads the child's response. The Carrier's hour
	// lifetime cannot make this two-second child writer or its cleanup survive.
	select {
	case err := <-written:
		if err == nil {
			t.Fatal("blocked child write succeeded")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("child write survived deadline")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("outer cleanup did not join child")
	}
}

func TestClosedForwardingInitialAcceptKeepsOperationDeadline(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	key := route.ClosedCarrierKey{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, LocalNodeID: [32]byte{3}, PeerNodeID: [32]byte{4}, PeerKey: [32]byte{5}, CarrierProfile: route.ClosedCarrierTCP}
	pool, err := route.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	lease, err := pool.Acquire(key, func() error { return nil }, func() (route.Carrier, error) { return local, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	read := make(chan error, 1)
	go func() { _, err := route.ReadClosedLaneFrame(peer); read <- err }()
	end := time.Now().UTC().Truncate(time.Second).Add(time.Hour)
	hello := func() (route.ClosedHello, error) {
		return route.ClosedHello{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4},
			RecipientNodeID: [32]byte{5}, RecipientDutyGeneration: 6, Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{7}, Deadline: end}, nil
	}
	result := make(chan error, 1)
	go func() {
		_, err := newClosedForwardingSessions(&sync.WaitGroup{}).acquire(key, lease, time.Now().Add(100*time.Millisecond), hello)
		result <- err
	}()
	if err := <-read; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("missing ACCEPT accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("initial ACCEPT inherited profile lifetime")
	}
}

func TestClosedForwardingBlockedWriteRetiresPhysicalCarrier(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	session := &closedForwardingSession{carrier: local}
	returned := make(chan error, 1)
	go func() {
		_, err := session.writeChildFrame(route.ClosedLaneFrame{Kind: 6, Lane: 1, Body: []byte{1}}, time.Now().Add(100*time.Millisecond), newClosedForwardingQueue(64))
		returned <- err
	}()
	select {
	case err := <-returned:
		if err == nil {
			t.Fatal("blocked writer succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("writer survived child deadline")
	}
	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil && err != io.ErrClosedPipe {
		t.Fatal(err)
	}
	if _, err := peer.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("partial-frame Carrier remained open: %v", err)
	}
}
