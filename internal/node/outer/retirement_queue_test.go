package outer

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// Hold only the return of a fully delivered frame. Parent retirement must
// discard a sibling's unemitted terminal while joining this original writer.
type retirementFrameReturn struct {
	net.Conn
	armed                    atomic.Bool
	written, release, closed chan struct{}
	once                     sync.Once
	lateErr                  error
}

func (connection *retirementFrameReturn) Write(body []byte) (int, error) {
	n, err := connection.Conn.Write(body)
	if connection.armed.CompareAndSwap(true, false) {
		close(connection.written)
		<-connection.release
		err = errors.Join(err, connection.lateErr)
	}
	return n, err
}

func (connection *retirementFrameReturn) Close() error {
	err := connection.Conn.Close()
	connection.once.Do(func() { close(connection.closed) })
	return err
}

func TestClosedOuterRetirementDiscardsUnemittedTerminalAndJoinsActiveWrite(t *testing.T) {
	for _, failWrite := range []bool{false, true} {
		t.Run(map[bool]string{false: "completed-frame", true: "failed-frame"}[failWrite], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				now := time.Now().UTC().Truncate(time.Second)
				receiver := route.ClosedOuterReceiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4},
					NodeID: [32]byte{5}, RecordDigest: [32]byte{6}, DutyGeneration: 7, RoleDomain: 2, Subrole: 6, Deadline: now.Add(time.Minute)}
				limits, err := route.NewClosedDutyLimits(time.Now)
				if err != nil {
					t.Fatal(err)
				}
				handshake, err := route.NewClosedOuterHandshake(receiver, limits, time.Now)
				if err != nil {
					t.Fatal(err)
				}
				local, peer := net.Pipe()
				defer local.Close()
				defer peer.Close()
				connection := &retirementFrameReturn{Conn: local, written: make(chan struct{}), release: make(chan struct{}), closed: make(chan struct{})}
				lateErr := errors.New("original active physical write failed")
				if failWrite {
					connection.lateErr = lateErr
				}
				var releaseOnce sync.Once
				defer releaseOnce.Do(func() { close(connection.release) })
				beginFrame, beginTerminal := make(chan struct{}), make(chan struct{})
				started := make(chan uint32, 2)
				terminal := make(chan error, 1)
				var sequence atomic.Uint32
				done := make(chan error, 1)
				go func() {
					done <- Serve(context.Background(), connection, handshake, func(_ context.Context, lane *route.ClosedOuterBridgeLane) {
						index := sequence.Add(1)
						started <- index
						if index == 1 {
							<-beginFrame
							_, _ = lane.Write([]byte{42})
						} else {
							<-beginTerminal
							terminal <- lane.CloseWithStatus(0)
						}
					})
				}()
				hello := ardp.Hello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest,
					RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{8}, Deadline: receiver.Deadline}
				body, err := ardp.EncodeHello(hello)
				if err != nil {
					t.Fatal(err)
				}
				if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindHello, Body: body}); err != nil {
					t.Fatal(err)
				}
				if frame, err := ardp.ReadFrame(peer); err != nil || frame.Kind != ardp.KindAccept {
					t.Fatalf("outer HELLO: %+v %v", frame, err)
				}
				body, err = route.EncodeClosedNodeOpen(route.ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeIssuer, Deadline: receiver.Deadline}, route.ClosedChildOrdinary)
				if err != nil {
					t.Fatal(err)
				}
				for _, id := range []uint32{1, 3} {
					if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindOpen, Lane: id, Body: body}); err != nil {
						t.Fatal(err)
					}
					<-started
				}
				connection.armed.Store(true)
				close(beginFrame)
				if frame, err := ardp.ReadFrame(peer); err != nil || frame.Kind != ardp.KindBytes || frame.Lane != 1 {
					t.Fatalf("original frame: %+v %v", frame, err)
				}
				<-connection.written
				close(beginTerminal)
				synctest.Wait()
				_ = peer.Close()
				<-connection.closed
				synctest.Wait()
				select {
				case err := <-done:
					t.Fatalf("Serve returned before original write joined: %v", err)
				default:
				}
				releaseOnce.Do(func() { close(connection.release) })
				if err := <-terminal; err == nil {
					t.Error("unemitted terminal survived parent retirement")
				}
				result := <-done
				if failWrite {
					if !errors.Is(result, lateErr) {
						t.Fatalf("original physical failure lost: %v", result)
					}
				} else if result != nil {
					t.Fatalf("unemitted terminal invented physical cleanup failure: %v", result)
				}
			})
		})
	}
}
