//go:build linux

package forwarding

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

func TestClosedForwardingLocalCloseAfterSelectedReverseCredit(t *testing.T) {
	end := time.Now().UTC().Truncate(time.Second).Add(8 * time.Second)
	open := route.ClosedOpen{NextNodeID: [32]byte{4}, NextDutyGeneration: 1, Purpose: ardp.PurposeIssuer, Deadline: end}
	channel := closedForwardingActualChannel(t, end, open)
	local, peer := net.Pipe()
	if err := local.SetDeadline(end); err != nil {
		t.Fatal(err)
	}
	if err := peer.SetDeadline(end); err != nil {
		t.Fatal(err)
	}
	pool, err := carrier.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	key := carrier.ClosedCarrierKey{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, LocalNodeID: [32]byte{3}, PeerNodeID: [32]byte{4}, PeerKey: [32]byte{5}, CarrierProfile: carrier.ClosedCarrierTCP}
	lease, err := pool.AcquireContext(t.Context(), key, func() error { return nil }, func() (carrier.Carrier, error) { return local, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = lease.MarkUsed(); err != nil {
		t.Fatal(err)
	}
	sibling, err := pool.AcquireContext(t.Context(), key, func() error { return nil }, func() (carrier.Carrier, error) { t.Error("shared Carrier redialed"); return nil, net.ErrClosed })
	if err != nil {
		t.Fatal(err)
	}
	if err = sibling.MarkUsed(); err != nil {
		t.Fatal(err)
	}
	reverse := newFrameQueue(128)
	credited := make(chan struct{})
	session := &session{owner: newSessionSet(), carrier: local, invalidate: func() error { return nil }, children: map[uint32]*frameQueue{3: reverse}, retired: make(map[uint32]struct{}), retirements: make(map[uint32]func() bool), queues: map[uint32]func(ardp.Frame) error{3: func(frame ardp.Frame) error {
		frame.Lane = 1
		err := channel.QueueReverse(frame)
		if err == nil && frame.Kind == ardp.KindCredit {
			close(credited)
		}
		return err
	}}}
	link := &forwardLink{session: session, remoteLane: 3, localLane: 1, reverse: reverse, lease: lease, channel: channel, deadline: end, done: make(chan struct{}), stopped: make(chan struct{}), abort: func() { t.Error("local child retirement aborted prefix") }, write: channel.AccountOutput}
	links := map[uint32]*forwardLink{1: link}
	sessionDone := make(chan struct{})
	go func() { defer close(sessionDone); session.copyReverse() }()
	frames := make(chan ardp.Frame, 3)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for range 3 {
			frame, err := ardp.ReadFrame(peer)
			if err != nil {
				return
			}
			frames <- frame
		}
	}()
	defer func() {
		_ = peer.Close()
		_ = local.Close()
		<-readerDone
		<-sessionDone
		link.stop()
		if err := sibling.Release(); err != nil {
			t.Error(err)
		}
		if err := pool.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err = channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{9}}); err != nil {
		t.Fatal(err)
	}
	server := &forwardServer{}
	if err = server.drainForwarding(t.Context(), channel, links, nil, channel.AccountOutput, func() { t.Error("unexpected abort") }); err != nil {
		t.Fatal(err)
	}
	select {
	case frame := <-frames:
		if frame.Kind != ardp.KindBytes || frame.Lane != 3 {
			t.Fatalf("initial output: %+v", frame)
		}
	case <-time.After(time.Second):
		t.Fatal("initial child output missing")
	}
	if err = ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindCredit, Lane: 3, Body: binary.BigEndian.AppendUint32(nil, 1)}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-credited:
	case <-time.After(time.Second):
		t.Fatal("genuine reverse CREDIT was not reserved")
	}
	if _, err = channel.Accept(ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{5}}); err != nil {
		t.Fatal(err)
	}
	go link.copyReverse()
	select {
	case <-link.done:
	case <-time.After(time.Second):
		t.Fatal("selected reverse CREDIT did not join local child retirement")
	}
	if reverse.peerClosed() {
		t.Fatal("local CLOSE fabricated a peer terminal witness")
	}
	if err = server.drainForwarding(t.Context(), channel, links, nil, channel.AccountOutput, func() { t.Error("local CLOSE aborted parent") }); err != nil {
		t.Fatalf("local CLOSE lost after reverse copier joined: %v", err)
	}
	select {
	case frame := <-frames:
		if frame.Kind != ardp.KindClose || frame.Lane != 3 || len(frame.Body) != 1 || frame.Body[0] != 5 {
			t.Fatalf("terminal output: %+v", frame)
		}
	case <-time.After(time.Second):
		t.Fatal("required outbound CLOSE missing")
	}
	body, err := route.EncodeClosedOpen(open)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 3, Body: body}); err != nil {
		t.Fatalf("local CLOSE poisoned live prefix: %v", err)
	}
	written, err := session.writeChildFrame(ardp.Frame{Kind: ardp.KindBytes, Lane: 5, Body: []byte("sibling")}, end, newFrameQueue(128))
	if !written || err != nil {
		t.Fatalf("local CLOSE poisoned shared Carrier: written=%t err=%v", written, err)
	}
	select {
	case frame := <-frames:
		if frame.Lane != 5 || string(frame.Body) != "sibling" {
			t.Fatalf("sibling output: %+v", frame)
		}
	case <-time.After(time.Second):
		t.Fatal("sibling output missing")
	}
}

func TestClosedForwardingLocalClosePreservesOnlyTerminalAuthority(t *testing.T) {
	for _, outcome := range []string{"close", "parent", "peer", "expiry"} {
		t.Run(outcome, func(t *testing.T) {
			local, peer := net.Pipe()
			session := &session{carrier: local}
			reverse := newFrameQueue(128)
			end := time.Now().Add(time.Second)
			if outcome == "expiry" {
				end = time.Now().Add(80 * time.Millisecond)
			}
			acquired, err := session.acquireWriter(t.Context(), time.Now().Add(time.Second), nil, false)
			if err != nil || !acquired {
				t.Fatalf("active writer unavailable: %t %v", acquired, err)
			}
			held := true
			var payload, terminal chan error
			payloadJoined, terminalJoined := false, false
			defer func() {
				reverse.close()
				_ = local.Close()
				_ = peer.Close()
				if held {
					session.releaseWriter()
				}
				if payload != nil && !payloadJoined {
					<-payload
				}
				if terminal != nil && !terminalJoined {
					<-terminal
				}
			}()
			payload = make(chan error, 1)
			go func() {
				written, err := session.writeChildFrame(ardp.Frame{Kind: ardp.KindBytes, Lane: 3, Body: []byte("obsolete")}, end, reverse)
				if written {
					t.Error("locally closed queued payload emitted")
				}
				payload <- err
			}()
			select {
			case err := <-payload:
				payloadJoined = true
				t.Fatalf("current payload escaped occupied writer: %v", err)
			case <-time.After(10 * time.Millisecond):
			}
			reverse.markLocalClose()
			select {
			case err := <-payload:
				payloadJoined = true
				if err != nil {
					t.Fatalf("local terminal poisoned unemitted payload: %v", err)
				}
			case <-time.After(100 * time.Millisecond):
				t.Fatal("local CLOSE did not cancel pending payload")
			}
			terminal = make(chan error, 1)
			go func() {
				written, err := session.writeChildFrame(ardp.Frame{Kind: ardp.KindClose, Lane: 3, Body: []byte{5}}, end, reverse)
				if outcome == "close" && !written {
					t.Error("required outbound CLOSE discarded")
				}
				if outcome != "close" && written {
					t.Error("terminal escaped retired authority")
				}
				terminal <- err
			}()
			if outcome == "close" {
				select {
				case err := <-terminal:
					terminalJoined = true
					t.Fatalf("local witness discarded required queued CLOSE: %v", err)
				case <-time.After(10 * time.Millisecond):
				}
				session.releaseWriter()
				held = false
				frame, err := ardp.ReadFrame(peer)
				if err != nil || frame.Kind != ardp.KindClose || frame.Lane != 3 {
					t.Fatalf("required terminal output: %+v %v", frame, err)
				}
				err = <-terminal
				terminalJoined = true
				if err != nil {
					t.Fatal(err)
				}
			} else {
				switch outcome {
				case "parent":
					reverse.close()
				case "peer":
					if err = reverse.push(ardp.Frame{Kind: ardp.KindClose, Lane: 3, Body: []byte{0}}, nil); err != nil {
						t.Fatal(err)
					}
				}
				select {
				case err := <-terminal:
					terminalJoined = true
					if outcome == "parent" && !errors.Is(err, context.Canceled) {
						t.Fatalf("parent cancellation lost: %v", err)
					}
					if outcome == "peer" && err != nil {
						t.Fatalf("peer terminal lost: %v", err)
					}
					if outcome == "expiry" && !errors.Is(err, context.DeadlineExceeded) {
						t.Fatalf("terminal deadline lost: %v", err)
					}
				case <-time.After(200 * time.Millisecond):
					t.Fatal("terminal survived its authority")
				}
				session.releaseWriter()
				held = false
				if err = peer.SetReadDeadline(time.Now().Add(15 * time.Millisecond)); err != nil {
					t.Fatal(err)
				}
				_, err = peer.Read(make([]byte, 1))
				var timeout net.Error
				if !errors.As(err, &timeout) || !timeout.Timeout() {
					t.Fatalf("retired frame emitted or healthy Carrier ended: %v", err)
				}
			}
			if err = peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			sibling := make(chan error, 1)
			go func() {
				_, err := session.writeChildFrame(ardp.Frame{Kind: ardp.KindBytes, Lane: 5, Body: []byte("sibling")}, time.Now().Add(time.Second), newFrameQueue(128))
				sibling <- err
			}()
			frame, readErr := ardp.ReadFrame(peer)
			writeErr := <-sibling
			if readErr != nil || frame.Lane != 5 || string(frame.Body) != "sibling" || writeErr != nil {
				t.Fatalf("late retired output or damaged sibling: %+v read=%v write=%v", frame, readErr, writeErr)
			}
		})
	}
}
