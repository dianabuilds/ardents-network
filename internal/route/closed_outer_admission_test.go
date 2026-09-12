package route

import (
	"bytes"
	"crypto/tls"
	"io"
	"testing"
	"time"
)

// Credential/exporter fixtures isolate real receiving admission, spend-ledger
// ownership and outer-lane lifetime. This does not qualify network credentials.
func closedOuterAdmissionFixture(t *testing.T) (*ClosedOuterBridgeLane, *ClosedOuterBridge, *ClosedAdmission, *time.Time) {
	t.Helper()
	return closedOuterAdmissionFixtureFor(t, ClosedPurposeForwarding, 2)
}

func closedOuterAdmissionFixtureFor(t *testing.T, purpose ClosedPurpose, class uint8) (*ClosedOuterBridgeLane, *ClosedOuterBridge, *ClosedAdmission, *time.Time) {
	t.Helper()
	now := time.Unix(1_800_000_000, 0).UTC()
	receiver := closedOuterHandshakeReceiver(now)
	receiver.RoleDomain, receiver.Subrole, receiver.Deadline = 1, 2, now.Add(time.Hour)
	if purpose == ClosedPurposeIssuer {
		receiver.RoleDomain, receiver.Subrole = 2, 6
	}
	if purpose == ClosedPurposeDataJoin {
		receiver.RoleDomain, receiver.Subrole = 2, 4
	}

	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	outer, err := NewClosedOuterHandshake(receiver, limits, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(outer.Close)
	bridge, err := NewClosedOuterBridge(outer, func(uint32, time.Time) error { return nil }, func(ClosedLaneFrame, func() time.Time) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	hello := ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest,
		ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration,
		Purpose: ClosedPurposeForwarding, ChannelNonce: [32]byte{19}, Deadline: receiver.Deadline}
	body, err := EncodeClosedHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameHello, Body: body}); err != nil {
		t.Fatal(err)
	}
	end := now.Add(30 * time.Minute)
	body, err = EncodeClosedNodeOpen(ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration,
		Purpose: purpose, Deadline: end}, ClosedChildOrdinary)
	if err != nil {
		t.Fatal(err)
	}
	lane, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 1, Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(lane, make([]byte, 1)); err != nil {
		t.Fatal(err)
	}
	if err := lane.BeginInnerHello(); err != nil {
		t.Fatal(err)
	}
	hello.ChannelNonce[0]++
	hello.Purpose = purpose
	hello.Deadline = end
	if err := lane.Activate(hello); err != nil {
		t.Fatal(err)
	}
	spends, err := OpenClosedSpendLedger(t.TempDir(), ClosedSpendBinding{NetworkID: receiver.NetworkID,
		ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := spends.Close(); err != nil {
			t.Error(err)
		}
	})
	role := ClosedRoleReceiver{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest,
		ProfileDigest: receiver.ProfileDigest, NodeID: receiver.NodeID, RecordDigest: receiver.RecordDigest,
		DutyGeneration: receiver.DutyGeneration, RoleDomain: receiver.RoleDomain, Subrole: receiver.Subrole, ExpectedPurpose: purpose, NotAfter: receiver.Deadline}
	admission, err := NewClosedAdmissionChannel(role, spends, limits,
		func(string, []byte, int) ([]byte, error) { return bytes.Repeat([]byte{1}, 32), nil },
		func(ClosedAdmissionVerification) (time.Time, error) { return now.Truncate(time.Hour), nil }, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	body, err = EncodeClosedHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admission.Accept(ClosedLaneFrame{Kind: closedFrameHello, Body: body}); err != nil {
		t.Fatal(err)
	}
	lease, err := admission.Accept(ClosedLaneFrame{Kind: closedFrameAdmit, Body: append([]byte{class}, bytes.Repeat([]byte{2}, 354)...)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(lease.Release)
	return lane, bridge, &lease, &now
}

func TestClosedOuterAdmissionExtendsOnlyItsVerifiedChild(t *testing.T) {
	lane, bridge, lease, now := closedOuterAdmissionFixture(t)
	pending := now.Add(10 * time.Second)
	if err := lane.SetDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	if got := lane.lane.currentWriteDeadline(); got != pending {
		t.Fatal("TLS deadline reset extended pending child")
	}
	if err := lane.admitVerified(lease); err != nil {
		t.Fatal(err)
	}
	if err := lane.SetDeadline(lease.Deadline.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := lane.lane.currentWriteDeadline(); got != lease.Deadline {
		t.Fatal("admitted deadline not bounded by actual lease")
	}
	*now = now.Add(11 * time.Second)
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{2}}); err != nil {
		t.Fatalf("admitted child lost after handshake interval: %v", err)
	}
	if err := lane.admitVerified(lease); err == nil {
		t.Fatal("admission reused")
	}
	*now = lease.Deadline
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{2}}); err == nil {
		t.Fatal("child outlived admitted lease")
	}
}

func TestClosedOuterAdmissionRefusesUnboundOrExpiredAuthority(t *testing.T) {
	for _, fault := range []string{"fabricated", "other-hello", "restricted", "expired-handshake", "unadmitted"} {
		t.Run(fault, func(t *testing.T) {
			lane, bridge, lease, now := closedOuterAdmissionFixture(t)
			switch fault {
			case "fabricated":
				lease = &ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: lease.Deadline}
			case "other-hello":
				lease.hello.ChannelNonce[0]++
			case "restricted":
				child := bridge.handshake.children[1]
				child.restriction = ClosedChildIssuerBootstrap
				bridge.handshake.children[1] = child
			case "expired-handshake", "unadmitted":
				*now = now.Add(10 * time.Second)
			}
			if err := lane.admitVerified(lease); err == nil {
				t.Fatal("invalid admission extended child")
			}
			if fault == "unadmitted" {
				if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{2}}); err == nil {
					t.Fatal("HELLO alone extended pending lane")
				}
			}
		})
	}
}

func TestClosedOuterAdmissionRequiresActualTLSOnThisLane(t *testing.T) {
	lane, _, lease, _ := closedOuterAdmissionFixture(t)
	other, _, _, _ := closedOuterAdmissionFixture(t)
	for _, connection := range []*tls.Conn{nil, tls.Server(other, &tls.Config{}), tls.Server(lane, &tls.Config{})} {
		if connection == nil {
			if err := lane.Admit(lease, nil); err == nil {
				t.Fatal("missing TLS accepted")
			}
		} else if err := lane.Admit(lease, connection); err == nil {
			t.Fatal("foreign or unhandshaken TLS accepted")
		}
	}
}

func TestClosedOuterRetirementDoesNotWaitForBlockedChildWrite(t *testing.T) {
	lane, bridge, _, _ := closedOuterAdmissionFixture(t)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	bridge.write = func(frame ClosedLaneFrame, _ func() time.Time) error {
		if frame.Kind == closedFrameBytes {
			close(entered)
			<-release
		}
		return nil
	}
	go func() { _, err := lane.Write([]byte{1}); done <- err }()
	<-entered
	retired := make(chan error, 1)
	go func() {
		_, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameClose, Lane: 1, Body: []byte{0}})
		retired <- err
	}()
	select {
	case err := <-retired:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Error("incoming CLOSE waited for blocked payload writer")
	}
	close(release)
	<-done
	if _, err := lane.Write([]byte{1}); err == nil {
		t.Fatal("retired child wrote again")
	}
}

func TestClosedOuterTerminalControlRemainsWritableAfterPayloadExpiry(t *testing.T) {
	lane, bridge, _, _ := closedOuterAdmissionFixture(t)
	lane.lane.hardDeadline = time.Now().Add(-time.Second)
	wrote := false
	bridge.write = func(frame ClosedLaneFrame, deadline func() time.Time) error {
		if frame.Kind != closedFrameClose {
			t.Fatal("expiry emitted payload")
		}
		end := deadline()
		if !end.After(time.Now()) || end.After(time.Now().Add(2*time.Second)) {
			t.Fatal("terminal deadline is expired or unbounded")
		}
		wrote = true
		return nil
	}
	if err := lane.Close(); err != nil {
		t.Fatal(err)
	}
	if !wrote {
		t.Fatal("no terminal CLOSE")
	}
}

func TestClosedOuterControlAdmissionUsesItsActualLifetime(t *testing.T) {
	lane, bridge, lease, now := closedOuterAdmissionFixtureFor(t, ClosedPurposeIssuer, 1)
	if lease.Deadline != now.Add(30*time.Second) || lease.Bytes != 64<<10 {
		t.Fatal("Control admission acquired forwarding limits")
	}
	if err := lane.admitVerified(lease); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(11 * time.Second)
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{2}}); err != nil {
		t.Fatalf("Control admission expired at handshake bound: %v", err)
	}
	*now = lease.Deadline
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{2}}); err == nil {
		t.Fatal("Control child outlived its thirty-second lease")
	}
}

func TestClosedOuterAdmissionRejectsClassForAnotherPurpose(t *testing.T) {
	for _, pair := range []struct {
		purpose ClosedPurpose
		class   uint8
	}{{ClosedPurposeIssuer, 2}, {ClosedPurposeForwarding, 1}, {ClosedPurposeIssuer, 3}} {
		lane, _, lease, _ := closedOuterAdmissionFixtureFor(t, pair.purpose, pair.class)
		if err := lane.admitVerified(lease); err == nil {
			t.Fatalf("class %d extended purpose %d", pair.class, pair.purpose)
		}
	}
}
