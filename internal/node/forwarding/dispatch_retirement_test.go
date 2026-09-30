package forwarding

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

func TestClosedForwardingSelectedCreditJoinsPeerClose(t *testing.T) {
	t.Run("authenticated-close", func(t *testing.T) { testSelectedCreditRetirement(t, true) })
	t.Run("physical-eof", func(t *testing.T) { testSelectedCreditRetirement(t, false) })
}

func testSelectedCreditRetirement(t *testing.T, peerClose bool) {
	clock := time.Now
	end := clock().UTC().Truncate(time.Second).Add(8 * time.Second)
	governor, err := route.NewClosedBootstrapController(clock)
	if err != nil {
		t.Fatal(err)
	}
	limits, err := route.NewClosedDutyLimits(clock)
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := governor.Admit([32]byte{1}, end)
	if err != nil {
		t.Fatal(err)
	}
	channel, err := route.NewClosedBootstrapForwardingChannel(reservation, limits, func(route.ClosedOpen) error { return nil }, clock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := channel.Cancel(); err != nil {
			t.Errorf("channel cleanup: %v", err)
		}
	}()
	body, err := route.EncodeClosedOpen(route.ClosedOpen{NextNodeID: [32]byte{3}, NextDutyGeneration: 4, Purpose: ardp.PurposeIssuer, Deadline: end})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	channel.NextAvailable(nil)
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	if err = local.SetDeadline(end); err != nil {
		t.Fatal(err)
	}
	if err = peer.SetDeadline(end); err != nil {
		t.Fatal(err)
	}
	pool, err := carrier.NewClosedCarrierPool(clock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := pool.Close(); err != nil {
			t.Errorf("pool cleanup: %v", err)
		}
	}()
	key := carrier.ClosedCarrierKey{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, LocalNodeID: [32]byte{3}, PeerNodeID: [32]byte{4}, PeerKey: [32]byte{5}, CarrierProfile: carrier.ClosedCarrierTCP}
	lease, err := pool.AcquireContext(t.Context(), key, func() error { return nil }, func() (carrier.Carrier, error) { return local, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = lease.MarkUsed(); err != nil {
		t.Fatal(err)
	}
	siblingLease, err := pool.AcquireContext(t.Context(), key, func() error { return nil }, func() (carrier.Carrier, error) { t.Error("sibling redialed shared Carrier"); return nil, net.ErrClosed })
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := siblingLease.Release(); err != nil {
			t.Errorf("sibling lease cleanup: %v", err)
		}
	}()
	if err = siblingLease.MarkUsed(); err != nil {
		t.Fatal(err)
	}
	reverse := newFrameQueue(128)
	session := &session{owner: newSessionSet(), carrier: local, invalidate: func() error { return nil }, children: map[uint32]*frameQueue{3: reverse}, retired: make(map[uint32]struct{}), retirements: make(map[uint32]func() bool), queues: map[uint32]func(ardp.Frame) error{3: func(frame ardp.Frame) error { frame.Lane = 1; return channel.QueueReverse(frame) }}}
	published := make(chan ardp.Frame, 2)
	aborted := make(chan struct{}, 1)
	link := &forwardLink{session: session, remoteLane: 3, localLane: 1, reverse: reverse, lease: lease, channel: channel, deadline: end, done: make(chan struct{}), stopped: make(chan struct{}), abort: func() { aborted <- struct{}{} }, write: func(frame ardp.Frame) error {
		if err := channel.AccountOutput(frame); err != nil {
			return err
		}
		published <- frame
		return nil
	}}
	sessionDone := make(chan struct{})
	go func() { session.copyReverse(); close(sessionDone) }()
	go link.copyReverse()
	defer func() {
		_ = local.Close()
		_ = peer.Close()
		link.stop()
		<-link.done
		<-sessionDone
		if link.stopErr != nil {
			t.Errorf("child cleanup: %v", link.stopErr)
		}
	}()
	if err = ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindBytes, Lane: 3, Body: []byte{7}}); err != nil {
		t.Fatal(err)
	}
	select {
	case frame := <-published:
		if frame.Kind != ardp.KindBytes {
			t.Fatal("response missing")
		}
	case <-time.After(time.Second):
		t.Fatal("response blocked")
	}
	if _, err = channel.Accept(ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: binary.BigEndian.AppendUint32(nil, 1)}); err != nil {
		t.Fatal(err)
	}
	event, available := nextAvailableForwarding(channel, map[uint32]*forwardLink{1: link}, nil)
	if !available || event.Kind != ardp.KindCredit {
		t.Fatal("live child CREDIT not selected")
	}
	if peerClose {
		if err = ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindClose, Lane: 3, Body: []byte{0}}); err != nil {
			t.Fatal(err)
		}
	} else if err = peer.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-link.done:
	case <-time.After(time.Second):
		t.Fatal("terminal child did not join")
	}
	if reverse.peerClosed() != peerClose {
		t.Fatal("physical EOF changed the terminal CLOSE witness")
	}
	if !peerClose {
		if link.startForwarding(event, func() {}, func() { t.Error("retired physical Carrier dispatched CREDIT") }) {
			t.Fatal("generic physical EOF normalized into successful retirement")
		}
		select {
		case <-aborted:
		default:
			t.Fatal("physical EOF did not abort parent")
		}
		return
	}
	select {
	case <-aborted:
		t.Fatal("peer terminal CLOSE aborted parent")
	default:
	}
	// The live Node consumer selects before the reverse copier joins this CLOSE.
	// Retirement in this gap must not indict the reusable parent Carrier.
	if !link.startForwarding(event, func() {}, func() { t.Error("late CREDIT aborted parent") }) {
		t.Fatal("selected CREDIT after authenticated peer CLOSE refused parent")
	}
	if err = link.forwardingError(); err != nil {
		t.Fatal(err)
	}
	sibling := newFrameQueue(128)
	peerRead := make(chan error, 1)
	peerReadDone := make(chan struct{})
	defer func() { _ = peer.Close(); <-peerReadDone }()
	go func() {
		defer close(peerReadDone)
		frame, readErr := ardp.ReadFrame(peer)
		if readErr == nil && (frame.Kind != ardp.KindBytes || frame.Lane != 5 || string(frame.Body) != "sibling") {
			readErr = net.ErrClosed
		}
		peerRead <- readErr
	}()
	written, writeErr := session.writeChildFrame(ardp.Frame{Kind: ardp.KindBytes, Lane: 5, Body: []byte("sibling")}, end, sibling)
	if writeErr != nil || !written {
		t.Fatalf("terminal child retired sibling Carrier: written=%t error=%v", written, writeErr)
	}
	if err = <-peerRead; err != nil {
		t.Fatal(err)
	}
}
