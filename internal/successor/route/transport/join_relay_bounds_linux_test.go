//go:build linux

package transport

import (
	"encoding/binary"
	"testing"
	"testing/synctest"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// These controls exercise actual post-admission frame I/O. They grant no
// Network, Admission, Hosting or Service authority; genuine composition is
// separately covered by the both-Carrier command scenarios.
func TestJoinRelayRefusesActivatedControlWithoutForwarding(t *testing.T) {
	request, err := ardp.EncodeJoinRequest(joinBoundsRequest(1, 9))
	if err != nil {
		t.Fatal(err)
	}
	for name, frame := range map[string]ardp.Frame{
		"lane-zero-refill": {Kind: ardp.KindAdmit, Body: make([]byte, 355)},
		"lane-one-refill":  {Kind: ardp.KindAdmit, Lane: 1, Body: make([]byte, 355)},
		"second-join":      {Kind: ardp.KindOperation, Lane: 1, Body: request},
		"unknown-lane":     {Kind: ardp.KindBytes, Lane: 3, Body: []byte{1}},
		"unearned-credit":  {Kind: ardp.KindCredit, Lane: 1, Body: binary.BigEndian.AppendUint32(nil, 1)},
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				fixture := newJoinBoundsFixture()
				defer fixture.close(t)
				first := fixture.start(t, joinBoundsRequest(1, 1), nil)
				second := fixture.start(t, joinBoundsRequest(2, 2), nil)
				readJoinBoundsResult(t, first, 1)
				readJoinBoundsResult(t, second, 2)
				written := make(chan error, 1)
				go func() { written <- ardp.WriteFrame(first.conn, frame) }()
				if forwarded, err := ardp.ReadFrame(second.conn); err == nil {
					t.Fatal("forbidden frame reached opposite side", forwarded)
				}
				<-written // an early header refusal can interrupt the body write
				if first.wait() == nil || second.wait() == nil {
					t.Fatal("forbidden frame became successful joined completion")
				}
				if fixture.queues.used != 2*joinFrameMemory || fixture.queues.children != 2 {
					t.Fatal("forbidden frame changed retained original capacity")
				}
			})
		})
	}
}

func TestJoinRelayRefusesBytesBeyondOriginalReceiveCredit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newJoinBoundsFixture()
		defer fixture.close(t)
		first := fixture.start(t, joinBoundsRequest(1, 1), nil)
		second := fixture.start(t, joinBoundsRequest(2, 2), nil)
		readJoinBoundsResult(t, first, 1)
		readJoinBoundsResult(t, second, 2)
		for range 4 {
			written := make(chan error, 1)
			go func() {
				written <- ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: make([]byte, 16384)})
			}()
			frame, err := ardp.ReadFrame(second.conn)
			if err != nil || frame.Kind != ardp.KindBytes || len(frame.Body) != 16384 {
				t.Fatal("original 64 KiB credit unavailable", err)
			}
			if err := <-written; err != nil {
				t.Fatal(err)
			}
		}
		written := make(chan error, 1)
		go func() {
			written <- ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{1}})
		}()
		if frame, err := ardp.ReadFrame(second.conn); err == nil {
			t.Fatal("uncredited fifth frame reached opposite side", frame)
		}
		<-written
		if first.wait() == nil || second.wait() == nil {
			t.Fatal("receive-credit exhaustion became successful completion")
		}
	})
}

func TestJoinCapacityAggregateRefusalRetainsOriginalBorrowers(t *testing.T) {
	queues := &queueBudget{maximum: 2 * joinFrameMemory}
	first, err := reserveJoinCapacity(queues)
	if err != nil {
		t.Fatal(err)
	}
	second, err := reserveJoinCapacity(queues)
	if err != nil {
		t.Fatal(err)
	}
	if third, err := reserveJoinCapacity(queues); err == nil || third != nil {
		t.Fatal("aggregate exhaustion acquired another side")
	}
	if queues.used != 2*joinFrameMemory || queues.children != 2 {
		t.Fatal("refused allocation released or leaked original capacity")
	}
	first.release()
	first.release()
	if queues.used != joinFrameMemory || queues.children != 1 {
		t.Fatal("repeated retirement reclaimed another borrower")
	}
	second.release()
	if queues.used != 0 || queues.children != 0 {
		t.Fatal("joined original capacity remained reserved")
	}
}

func TestJoinRelayOriginalByteAllowanceCountsSetupAndHeaders(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newJoinBoundsFixture()
		defer fixture.close(t)
		// Independent wire accounting: HELLO/ADMIT/ACCEPT, JOIN and RESULT,
		// one single-byte data frame, then only the next header can fit.
		const limit = (3*16 + 209 + 355 + 5) + (16 + 4096) + (16 + 16384) + (16 + 1) + 16
		first := fixture.startWithBounds(t, joinBoundsRequest(1, 1), nil, [32]byte{3}, limit)
		second := fixture.start(t, joinBoundsRequest(2, 2), nil)
		readJoinBoundsResult(t, first, 1)
		readJoinBoundsResult(t, second, 2)
		written := make(chan error, 1)
		go func() {
			written <- ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{7}})
		}()
		frame, err := ardp.ReadFrame(second.conn)
		if err != nil || frame.Kind != ardp.KindBytes || len(frame.Body) != 1 || frame.Body[0] != 7 {
			t.Fatal("original remaining allowance was unavailable", frame, err)
		}
		if err := <-written; err != nil {
			t.Fatal(err)
		}
		go func() {
			written <- ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{8}})
		}()
		if frame, err := ardp.ReadFrame(second.conn); err == nil {
			t.Fatal("bytes beyond original allowance were forwarded", frame)
		}
		<-written
		if first.wait() == nil || second.wait() == nil {
			t.Fatal("byte exhaustion became successful completion")
		}
		if fixture.queues.used != 2*joinFrameMemory || fixture.queues.children != 2 {
			t.Fatal("byte refusal reclaimed still-owned capacity")
		}
	})
}
