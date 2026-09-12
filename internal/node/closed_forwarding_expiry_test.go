package node

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// The clock and admission are fixtures. The shared physical reader, queue
// reservation and sibling delivery use their production owners. Expiry is
// local retirement, not evidence that the shared peer violated its protocol.
func TestClosedForwardingExpiredParentCannotRetireSharedCarrier(t *testing.T) {
	for _, scenario := range []struct {
		name              string
		full, duringCheck bool
	}{{name: "empty"}, {name: "full", full: true}, {name: "expires-after-check", full: true, duringCheck: true}} {
		t.Run(scenario.name, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			clock := func() time.Time { return now }
			limits, err := route.NewClosedDutyLimits(clock)
			if err != nil {
				t.Fatal(err)
			}
			governor, err := route.NewClosedBootstrapController(clock)
			if err != nil {
				t.Fatal(err)
			}
			var channels [2]*route.ClosedForwardingChannel
			for index, lifetime := range []time.Duration{2 * time.Second, 8 * time.Second} {
				end := now.Add(lifetime)
				reservation, err := governor.Admit([32]byte{byte(index + 1)}, end)
				if err != nil {
					t.Fatal(err)
				}
				channels[index], err = route.NewClosedBootstrapForwardingChannel(reservation, limits, func(route.ClosedOpen) error { return nil }, clock)
				if err != nil {
					t.Fatal(err)
				}
				defer channels[index].Cancel()
				body, err := route.EncodeClosedOpen(route.ClosedOpen{NextNodeID: [32]byte{3}, NextDutyGeneration: 4, Purpose: route.ClosedPurposeIssuer, Deadline: end})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := channels[index].Accept(route.ClosedLaneFrame{Kind: 4, Lane: 1, Body: body}); err != nil {
					t.Fatal(err)
				}
				if _, ok := channels[index].Next(); !ok {
					t.Fatal("opening was not scheduled")
				}
			}
			first, second := newClosedForwardingQueue(80), newClosedForwardingQueue(80)
			local, peer := net.Pipe()
			session := &closedForwardingSession{owner: newClosedForwardingSessions(&sync.WaitGroup{}), carrier: local,
				invalidate: func() error { return nil }, retired: make(map[uint32]struct{}),
				children: map[uint32]*closedForwardingQueue{1: first, 3: second},
				queues: map[uint32]func(route.ClosedLaneFrame) error{
					1: func(frame route.ClosedLaneFrame) error { return channels[0].QueueReverse(frame) },
					3: func(frame route.ClosedLaneFrame) error { frame.Lane = 1; return channels[1].QueueReverse(frame) },
				}, retirements: map[uint32]func() bool{1: func() bool { return channels[0].ReverseRetired(1) }, 3: func() bool { return channels[1].ReverseRetired(1) }}}
			if scenario.full {
				for range 4 {
					if !session.deliverReverse(route.ClosedLaneFrame{Kind: 6, Lane: 1, Body: []byte("full")}) {
						t.Fatal("live queue refused")
					}
				}
			}
			// The first check may observe live authority immediately before the
			// deadline expires and the physical queue rejects the late frame.
			if scenario.duringCheck {
				previous := session.retirements[1]
				var advance sync.Once
				session.retirements[1] = func() bool {
					retired := previous()
					advance.Do(func() { now = now.Add(3 * time.Second) })
					return retired
				}
			} else {
				now = now.Add(3 * time.Second)
			}
			finished := make(chan struct{})
			go func() { defer close(finished); session.copyReverse() }()
			defer func() {
				if err := peer.Close(); err != nil {
					t.Error(err)
				}
				if err := local.Close(); err != nil {
					t.Error(err)
				}
				<-finished
			}()
			if err := peer.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if err := route.WriteClosedLaneFrame(peer, route.ClosedLaneFrame{Kind: 9, Lane: 1, Body: []byte{1}}); err != nil {
				t.Fatal(err)
			}
			if err := route.WriteClosedLaneFrame(peer, route.ClosedLaneFrame{Kind: 6, Lane: 3, Body: []byte("sibling")}); err != nil {
				t.Fatalf("expired child closed shared Carrier before live sibling: %v", err)
			}
			frame, ok := second.next()
			if !ok || frame.Kind != 6 || string(frame.Body) != "sibling" {
				t.Fatal("live sibling response lost")
			}
			frame.Lane = 1
			if err := channels[1].AccountOutput(frame); err != nil {
				t.Fatal(err)
			}
			channels[1].ReleaseReverse(1, uint64(16+len(frame.Body)))
		})
	}
}
