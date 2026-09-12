package node

import (
	"bytes"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// State acceptance and pending acknowledgements are explicit seams. This
// exercises the actual delivery writer, queue, nonce and byte reservations.
func TestClosedIntroductionDeliverySerializesIDsAndBoundsPending(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	now := time.Now().UTC().Truncate(time.Second)
	var clock atomic.Int64
	clock.Store(now.Unix())
	slot := &closedIntroductionSlot{request: route.ClosedRegistrationRequest{Slot: [32]byte{1}, Revision: 1, Expiry: now.Add(30 * time.Second)},
		connection: local, writer: make(chan struct{}, 1), done: make(chan struct{}), active: true, maximum: 1 << 20,
		pending: make(map[uint32]*closedIntroductionDelivery)}
	server := &closedIntroductionServer{config: runtimeConfig{Config: Config{Current: func() (DutyView, error) {
		return nil, errors.New("State intentionally unavailable at final acknowledgement")
	}},
		now: func() time.Time { return time.Unix(clock.Load(), 0) }}, slots: map[[32]byte]*closedIntroductionSlot{slot.request.Slot: slot}}
	capsule := route.ClosedIntroductionCapsule{Slot: slot.request.Slot, Revision: 1, Expiry: now.Add(10 * time.Second), DeliveryNonce: [32]byte{2}, Encapsulation: [32]byte{3}, Ciphertext: bytes.Repeat([]byte{4}, 360)}
	slot.writer <- struct{}{}
	released := false
	defer func() {
		if !released {
			<-slot.writer
		}
		server.interruptSlot(slot)
	}()
	results := make(chan uint8, 16)
	for batch := 0; batch < 4; batch++ {
		clock.Store(now.Add(time.Duration(batch) * time.Second).Unix())
		for i := 0; i < 4; i++ {
			go func() { results <- server.deliver(t.Context(), capsule) }()
		}
		deadline := time.Now().Add(2 * time.Second)
		for {
			server.slotsMu.Lock()
			pending, next := slot.inFlight, slot.next
			server.slotsMu.Unlock()
			if next != 0 {
				t.Fatal("lane ID allocated before owning its writer")
			}
			if pending == (batch+1)*4 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("delivery reservation did not arrive")
			}
			time.Sleep(time.Millisecond)
		}
		if status := server.deliver(t.Context(), capsule); status != 1 {
			t.Fatal("rate/pending ceiling exceeded")
		}
	}
	server.slotsMu.Lock()
	if slot.used != 16*closedIntroductionDeliveryBytes {
		t.Error("queued work escaped shared byte budget")
	}
	server.slotsMu.Unlock()
	clock.Store(now.Add(4 * time.Second).Unix())
	if status := server.deliver(t.Context(), capsule); status != 1 {
		t.Fatal("17th pending capsule accepted")
	}
	<-slot.writer
	released = true
	if err := peer.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var last uint32
	operations, closed := 0, 0
	nonces := make(map[[32]byte]bool)
	for closed < 4 {
		frame, err := route.ReadClosedLaneFrame(peer)
		if err != nil {
			t.Fatal(err)
		}
		if frame.Kind == 10 {
			operations++
			if frame.Lane <= last || frame.Lane%2 != 0 {
				t.Fatal("concurrent delivery IDs arrived out of order")
			}
			last = frame.Lane
			nonce, received, err := route.DecodeClosedIntroductionSubmission(frame.Body)
			if err != nil {
				t.Fatal(err)
			}
			if nonce == capsule.DeliveryNonce || nonces[nonce] {
				t.Fatal("delivery reused a non-channel-local nonce")
			}
			nonces[nonce] = true
			if received.Slot != capsule.Slot || received.DeliveryNonce != capsule.DeliveryNonce || received.Encapsulation != capsule.Encapsulation || !bytes.Equal(received.Ciphertext, capsule.Ciphertext) {
				t.Fatal("Introduction changed sealed capsule")
			}
			server.slotsMu.Lock()
			pending := slot.pending[frame.Lane]
			if pending == nil || pending.nonce != nonce {
				server.slotsMu.Unlock()
				t.Fatal("acknowledgement uses wrong channel nonce")
			}
			pending.result <- 1
			server.slotsMu.Unlock()
		} else if frame.Kind == 9 && frame.Body[0] == 1 {
			closed++
		} else {
			t.Fatalf("unexpected delivery frame %d", frame.Kind)
		}
	}
	if operations != 4 {
		t.Fatal("queued deliveries escaped the actual dispatch rate")
	}
	for i := 0; i < 16; i++ {
		if <-results != 1 {
			t.Fatal("unavailable final State accepted delivery")
		}
	}
	server.slotsMu.Lock()
	defer server.slotsMu.Unlock()
	if slot.inFlight != 0 || len(slot.pending) != 0 {
		t.Fatal("completed delivery retained pending ownership")
	}
}
