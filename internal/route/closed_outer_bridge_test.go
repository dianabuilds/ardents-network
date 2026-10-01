package route

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func TestClosedOuterBridgeCarriesOpaqueInnerLaneWithCredit(t *testing.T) {
	now := time.Unix(1_800_300_000, 0).UTC()
	receiver := closedOuterHandshakeReceiver(now)
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handshake, err := NewClosedOuterHandshake(receiver, limits, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer handshake.Close()
	written := make(chan ardp.Frame, 8)
	bridge, err := NewClosedOuterBridge(handshake, func(uint32, time.Time) error { return nil }, func(frame ardp.Frame, _ func() time.Time, _, _ bool) error { written <- frame; return nil })
	if err != nil {
		t.Fatal(err)
	}
	hello := ardp.Hello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest,
		ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration,
		Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{19}, Deadline: receiver.Deadline}
	helloBody, err := ardp.EncodeHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if lane, err := bridge.Accept(ardp.Frame{Kind: ardp.KindHello, Body: helloBody}); err != nil || lane != nil {
		t.Fatalf("outer HELLO = %v / %v", lane, err)
	}
	if accepted := <-written; accepted.Kind != ardp.KindAccept || accepted.Lane != 0 {
		t.Fatalf("outer accept = %+v", accepted)
	}
	openBody, err := EncodeClosedNodeOpen(ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeIssuer, Deadline: receiver.Deadline}, ClosedChildOrdinary)
	if err != nil {
		t.Fatal(err)
	}
	lane, err := bridge.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 1, Body: openBody})
	if err != nil || lane == nil {
		t.Fatalf("outer open = %v / %v", lane, err)
	}
	if _, err := bridge.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{1, 2, 3}}); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 3)
	if count, err := io.ReadFull(lane, buffer); err != nil || !bytes.Equal(buffer, []byte{1, 2, 3}) || count != 3 {
		t.Fatalf("pre-TLS bridge read = %d / %x / %v", count, buffer, err)
	}
	if err := lane.BeginInnerHello(); err != nil {
		t.Fatal(err)
	}
	inner := hello
	inner.Purpose, inner.ChannelNonce = ardp.PurposeIssuer, [32]byte{20}
	if _, err := bridge.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{4, 5}}); err != nil {
		t.Fatal(err)
	}
	// The sender may pipeline encrypted admission immediately after HELLO.
	if err := lane.Activate(inner); err != nil {
		t.Fatal(err)
	}
	buffer = make([]byte, 2)
	if count, err := io.ReadFull(lane, buffer); err != nil || !bytes.Equal(buffer, []byte{4, 5}) || count != 2 {
		t.Fatalf("post-TLS bridge read = %d / %x / %v", count, buffer, err)
	}
	if credit := <-written; credit.Kind != ardp.KindCredit || credit.Lane != 1 || !bytes.Equal(credit.Body, []byte{0, 0, 0, 2}) {
		t.Fatalf("inner read credit = %+v", credit)
	}
	if count, err := lane.Write([]byte{6, 7}); err != nil || count != 2 {
		t.Fatalf("inner write = %d / %v", count, err)
	}
	if outbound := <-written; outbound.Kind != ardp.KindBytes || outbound.Lane != 1 || !bytes.Equal(outbound.Body, []byte{6, 7}) {
		t.Fatalf("inner outbound bytes = %+v", outbound)
	}
	creditBody := make([]byte, 4)
	binary.BigEndian.PutUint32(creditBody, 2)
	if _, err := bridge.Accept(ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: creditBody}); err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Accept(ardp.Frame{Kind: ardp.KindEOF, Lane: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := lane.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("inner EOF = %v", err)
	}
	if err := lane.CloseWithStatus(0); err != nil {
		t.Fatal(err)
	}
	if closed := <-written; closed.Kind != ardp.KindClose || closed.Lane != 1 || !bytes.Equal(closed.Body, []byte{0}) {
		t.Fatalf("local lane close = %+v", closed)
	}
	if replacement, err := bridge.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 3, Body: openBody}); err != nil || replacement == nil {
		t.Fatalf("released child replacement = %v / %v", replacement, err)
	}
}

func TestClosedOuterBridgeCloseDoesNotInterruptActiveCredit(t *testing.T) {
	now := time.Unix(1_800_300_000, 0).UTC()
	receiver := closedOuterHandshakeReceiver(now)
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handshake, err := NewClosedOuterHandshake(receiver, limits, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer handshake.Close()
	creditEntered := make(chan struct{})
	releaseCredit := make(chan struct{})
	creditFinished := make(chan struct{})
	deadlineUpdated := make(chan time.Time, 1)
	closeAttempted := make(chan struct{})
	closeWritten := make(chan struct{})
	closeFailure := errors.New("terminal output failed")
	var writer sync.Mutex
	bridge, err := NewClosedOuterBridge(handshake, func(_ uint32, end time.Time) error {
		deadlineUpdated <- end
		return nil
	}, func(frame ardp.Frame, _ func() time.Time, _, _ bool) error {
		if frame.Kind == ardp.KindClose {
			close(closeAttempted)
		}
		writer.Lock()
		defer writer.Unlock()
		switch frame.Kind {
		case ardp.KindCredit:
			close(creditEntered)
			<-releaseCredit
			close(creditFinished)
		case ardp.KindClose:
			close(closeWritten)
			return closeFailure
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	hello := ardp.Hello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest,
		ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration,
		Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{31}, Deadline: receiver.Deadline}
	helloBody, err := ardp.EncodeHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Accept(ardp.Frame{Kind: ardp.KindHello, Body: helloBody}); err != nil {
		t.Fatal(err)
	}
	openBody, err := EncodeClosedNodeOpen(ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeIssuer, Deadline: receiver.Deadline}, ClosedChildOrdinary)
	if err != nil {
		t.Fatal(err)
	}
	lane, err := bridge.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 1, Body: openBody})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	var preface [1]byte
	if _, err := io.ReadFull(lane, preface[:]); err != nil {
		t.Fatal(err)
	}
	if err := lane.BeginInnerHello(); err != nil {
		t.Fatal(err)
	}
	inner := hello
	inner.Purpose, inner.ChannelNonce = ardp.PurposeIssuer, [32]byte{32}
	if err := lane.Activate(inner); err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{7}}); err != nil {
		t.Fatal(err)
	}
	read := make(chan error, 1)
	go func() { var value [1]byte; _, err := lane.Read(value[:]); read <- err }()
	<-creditEntered
	closed := make(chan error, 1)
	closeStarted := time.Now()
	go func() { closed <- lane.CloseWithStatus(0) }()
	select {
	case <-closeAttempted:
	case err := <-closed:
		t.Fatalf("local CLOSE returned before active CREDIT completed: %v", err)
	}
	select {
	case end := <-deadlineUpdated:
		if end.Before(closeStarted) || end.After(closeStarted.Add(time.Second+100*time.Millisecond)) {
			t.Fatalf("active CREDIT cleanup deadline = %v after close start %v", end, closeStarted)
		}
	default:
		t.Fatal("local CLOSE did not bound the active CREDIT")
	}
	accepted := make(chan error, 1)
	go func() {
		_, err := bridge.Accept(ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: []byte{0, 0, 0, 1}})
		accepted <- err
	}()
	select {
	case err := <-accepted:
		if err != nil {
			t.Fatalf("in-flight retired frame = %v", err)
		}
	case <-time.After(time.Second):
		close(releaseCredit)
		<-read
		t.Fatal("local CLOSE held bridge admission while waiting for physical output")
	}
	joinedClose := make(chan error, 1)
	go func() { joinedClose <- lane.CloseWithStatus(0) }()
	select {
	case err := <-joinedClose:
		close(releaseCredit)
		<-read
		t.Fatalf("concurrent Close returned before owned cleanup: %v", err)
	default:
	}
	select {
	case err := <-closed:
		close(releaseCredit)
		<-read
		t.Fatalf("local CLOSE returned while active CREDIT remained blocked: %v", err)
	default:
	}
	close(releaseCredit)
	if err := <-read; err != nil {
		t.Fatal(err)
	}
	if err := <-closed; !errors.Is(err, closeFailure) {
		t.Fatalf("first Close result = %v", err)
	}
	if err := <-joinedClose; !errors.Is(err, closeFailure) {
		t.Fatalf("joined Close result = %v", err)
	}
	select {
	case <-closeWritten:
	default:
		t.Fatal("local CLOSE frame was not serialized after CREDIT")
	}
}

func TestClosedOuterBridgeRefusesForeignOpenBeforeCreatingLane(t *testing.T) {
	now := time.Unix(1_800_300_000, 0).UTC()
	receiver := closedOuterHandshakeReceiver(now)
	limits, _ := NewClosedDutyLimits(func() time.Time { return now })
	handshake, err := NewClosedOuterHandshake(receiver, limits, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer handshake.Close()
	bridge, err := NewClosedOuterBridge(handshake, func(uint32, time.Time) error { return nil }, func(ardp.Frame, func() time.Time, bool, bool) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	hello := ardp.Hello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest,
		RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{21}, Deadline: receiver.Deadline}
	body, err := ardp.EncodeHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Accept(ardp.Frame{Kind: ardp.KindHello, Body: body}); err != nil {
		t.Fatal(err)
	}
	foreign := receiver.NodeID
	foreign[0]++
	body, err = EncodeClosedNodeOpen(ClosedOpen{NextNodeID: foreign, NextDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeIssuer, Deadline: receiver.Deadline}, ClosedChildOrdinary)
	if err != nil {
		t.Fatal(err)
	}
	if lane, err := bridge.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 1, Body: body}); err == nil || lane != nil || len(bridge.lanes) != 0 {
		t.Fatalf("foreign open created bridge work: %v / %v / %d", lane, err, len(bridge.lanes))
	}
}
