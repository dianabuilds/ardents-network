package node

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestClosedOuterLifetimeInterruptsAndJoinsAllChildren(t *testing.T) {
	for _, ending := range []string{"peer-close", "cancel", "invalid-frame"} {
		t.Run(ending, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			receiver := route.ClosedOuterReceiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4},
				NodeID: [32]byte{5}, RecordDigest: [32]byte{6}, DutyGeneration: 7, RoleDomain: 2, Subrole: 6, Deadline: now.Add(10 * time.Second)}
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
			observed := &closedOuterObservedWriter{Conn: local, writing: make(chan struct{})}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			started, outcomes := make(chan struct{}, 3), make(chan error, 3)
			begin, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			defer releaseOnce.Do(func() { close(release) })
			var sequence, finished atomic.Uint32
			done := make(chan uint32, 1)
			go func() {
				serveClosedOuter(ctx, observed, outer, func(child context.Context, lane *route.ClosedOuterBridgeLane) {
					index := sequence.Add(1)
					started <- struct{}{}
					<-begin
					var err error
					switch index {
					case 1:
						_, err = lane.Read(make([]byte, 1))
						if !errors.Is(err, io.EOF) {
							outcomes <- errors.New("child reader did not receive terminal EOF")
							<-release
							finished.Add(1)
							return
						}
					case 2:
						_, err = lane.Write([]byte{1})
					case 3:
						<-child.Done()
						err = child.Err()
					}
					if err == nil {
						outcomes <- errors.New("child operation survived termination")
					} else {
						outcomes <- nil
					}
					<-release
					finished.Add(1)
				})
				done <- finished.Load()
			}()
			hello := route.ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest,
				RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{8}, Deadline: receiver.Deadline}
			body, err := route.EncodeClosedHello(hello)
			if err != nil {
				t.Fatal(err)
			}
			if err := route.WriteClosedLaneFrame(peer, route.ClosedLaneFrame{Kind: 1, Body: body}); err != nil {
				t.Fatal(err)
			}
			if frame, err := route.ReadClosedLaneFrame(peer); err != nil || frame.Kind != 5 {
				t.Fatalf("outer HELLO: %+v %v", frame, err)
			}
			body, err = route.EncodeClosedNodeOpen(route.ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration, Purpose: route.ClosedPurposeIssuer, Deadline: receiver.Deadline}, route.ClosedChildOrdinary)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []uint32{1, 3, 5} {
				if err := route.WriteClosedLaneFrame(peer, route.ClosedLaneFrame{Kind: 4, Lane: id, Body: body}); err != nil {
					t.Fatal(err)
				}
				select {
				case <-started:
				case <-time.After(5 * time.Second):
					t.Fatal("child not started")
				}
			}
			close(begin)
			select {
			case <-observed.writing:
			case <-time.After(5 * time.Second):
				t.Fatal("child write not reached")
			}
			switch ending {
			case "peer-close":
				_ = peer.Close()
			case "cancel":
				cancel()
			case "invalid-frame":
				if err := route.WriteClosedLaneFrame(peer, route.ClosedLaneFrame{Kind: 3, Body: []byte{2}}); err != nil {
					t.Fatal(err)
				}
			}
			for range 3 {
				select {
				case err := <-outcomes:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("child operation not interrupted")
				}
			}
			select {
			case count := <-done:
				t.Fatalf("outer returned before child cleanup (%d)", count)
			default:
			}
			releaseOnce.Do(func() { close(release) })
			select {
			case count := <-done:
				if count != 3 {
					t.Fatalf("joined %d children", count)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("outer did not join")
			}
			if _, err := outer.Accept(route.ClosedLaneFrame{Kind: 4, Lane: 7, Body: body}); err == nil {
				t.Fatal("retired outer allocated another child")
			}
		})
	}
}

type closedOuterObservedWriter struct {
	net.Conn
	writing chan struct{}
	once    sync.Once
}

func (connection *closedOuterObservedWriter) Write(value []byte) (int, error) {
	if len(value) >= 7 && string(value[:4]) == "ARDP" && value[6] == 6 {
		connection.once.Do(func() { close(connection.writing) })
	}
	return connection.Conn.Write(value)
}
