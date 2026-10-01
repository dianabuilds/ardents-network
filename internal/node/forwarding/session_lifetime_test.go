package forwarding

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// closedForwardingBlockedCloseCarrier makes cancellation observable before it
// interrupts an in-flight Carrier read. It keeps the callback alive until the
// test permits physical closure.
type closedForwardingBlockedCloseCarrier struct {
	routecarrier.Carrier
	closeEntered chan struct{}
	allowClose   <-chan struct{}
	done         chan struct{}
	once         sync.Once
	err          error
}

func (carrier *closedForwardingBlockedCloseCarrier) Close() error {
	carrier.once.Do(func() {
		close(carrier.closeEntered)
		<-carrier.allowClose
		carrier.err = carrier.Carrier.Close()
		close(carrier.done)
	})
	<-carrier.done
	return carrier.err
}

func TestClosedForwardingSessionCanceledLateAcceptWaitsForCloseAndReturnsNoSession(t *testing.T) {
	local, peer := net.Pipe()
	allowClose := make(chan struct{})
	carrier := &closedForwardingBlockedCloseCarrier{
		Carrier: local, closeEntered: make(chan struct{}), allowClose: allowClose, done: make(chan struct{}),
	}
	var workers sync.WaitGroup
	var allowOnce sync.Once
	releaseClose := func() { allowOnce.Do(func() { close(allowClose) }) }
	key := routecarrier.ClosedCarrierKey{NetworkID: [32]byte{31}, ProfileDigest: [32]byte{32}, LocalNodeID: [32]byte{33}, PeerNodeID: [32]byte{34}, PeerKey: [32]byte{35}, CarrierProfile: routecarrier.ClosedCarrierTCP}
	pool, err := routecarrier.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := pool.AcquireContext(t.Context(), key, func() error { return nil }, func() (routecarrier.Carrier, error) { return carrier, nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		releaseClose()
		_ = local.Close()
		_ = peer.Close()
		_ = lease.Release()
		_ = pool.Close()
		workers.Wait()
	})
	accept := mustClosedForwardAccept(t)
	helloRead := make(chan error, 1)
	allowAccept := make(chan struct{})
	var acceptOnce sync.Once
	releaseAccept := func() { acceptOnce.Do(func() { close(allowAccept) }) }
	acceptWritten := make(chan error, 1)
	t.Cleanup(releaseAccept)
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, readErr := ardp.ReadFrame(peer)
		helloRead <- readErr
		if readErr != nil {
			return
		}
		<-allowAccept
		acceptWritten <- ardp.WriteFrame(peer, accept)
	}()
	sessions := newSessionSet()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	type outcome struct {
		session *session
		err     error
	}
	creator := make(chan outcome, 1)
	deadline := time.Now().Add(5 * time.Second)
	helloDeadline := time.Now().UTC().Truncate(time.Second).Add(time.Minute)
	workers.Add(1)
	go func() {
		defer workers.Done()
		session, acquireErr := sessions.acquire(ctx, key, lease, deadline, func() (ardp.Hello, error) {
			return ardp.Hello{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, RecipientNodeID: [32]byte{5}, RecipientDutyGeneration: 6, Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{7}, Deadline: helloDeadline}, nil
		})
		creator <- outcome{session, acquireErr}
	}()
	select {
	case readErr := <-helloRead:
		if readErr != nil {
			t.Fatalf("outer HELLO read = %v", readErr)
		}
	case <-time.After(time.Second):
		t.Fatal("outer HELLO was not observed")
	}
	cancel()
	select {
	case <-carrier.closeEntered:
	case <-time.After(time.Second):
		t.Fatal("creator cancellation did not start Carrier close")
	}
	releaseAccept()
	select {
	case writeErr := <-acceptWritten:
		if writeErr != nil {
			t.Fatalf("late ACCEPT write = %v", writeErr)
		}
	case <-time.After(time.Second):
		t.Fatal("late ACCEPT was not consumed")
	}
	select {
	case got := <-creator:
		t.Fatalf("creator returned before cancellation close joined: %+v", got)
	case <-time.After(100 * time.Millisecond):
	}
	releaseClose()
	select {
	case got := <-creator:
		if got.session != nil || !errors.Is(got.err, context.Canceled) {
			t.Fatalf("canceled late ACCEPT creator = session %p error %v", got.session, got.err)
		}
	case <-time.After(time.Second):
		t.Fatal("creator did not return after cancellation close joined")
	}
	sessions.mu.Lock()
	_, pending := sessions.pending[key]
	_, published := sessions.sessions[key]
	sessions.mu.Unlock()
	if pending || published {
		t.Fatal("canceled late ACCEPT published a session")
	}
}

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
				body, err := route.EncodeClosedOpen(route.ClosedOpen{NextNodeID: [32]byte{3}, NextDutyGeneration: 4, Purpose: ardp.PurposeIssuer, Deadline: end})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := channels[index].Accept(ardp.Frame{Kind: 4, Lane: 1, Body: body}); err != nil {
					t.Fatal(err)
				}
				if _, ok := channels[index].NextAvailable(nil); !ok {
					t.Fatal("opening was not scheduled")
				}
			}
			first, second := newFrameQueue(80), newFrameQueue(80)
			local, peer := net.Pipe()
			session := &session{owner: newSessionSet(), carrier: local,
				invalidate: func() error { return nil }, retired: make(map[uint32]struct{}),
				children: map[uint32]*frameQueue{1: first, 3: second},
				queues: map[uint32]func(ardp.Frame) error{
					1: func(frame ardp.Frame) error { return channels[0].QueueReverse(frame) },
					3: func(frame ardp.Frame) error { frame.Lane = 1; return channels[1].QueueReverse(frame) },
				}, retirements: map[uint32]func() bool{1: func() bool { return channels[0].ReverseRetired(1) }, 3: func() bool { return channels[1].ReverseRetired(1) }}}
			if scenario.full {
				for range 4 {
					if !session.deliverReverse(ardp.Frame{Kind: 6, Lane: 1, Body: []byte("full")}) {
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
			if err := ardp.WriteFrame(peer, ardp.Frame{Kind: 9, Lane: 1, Body: []byte{1}}); err != nil {
				t.Fatal(err)
			}
			if err := ardp.WriteFrame(peer, ardp.Frame{Kind: 6, Lane: 3, Body: []byte("sibling")}); err != nil {
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
			channels[1].ReleaseReverse(frame)
		})
	}
}
