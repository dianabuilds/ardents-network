//go:build linux

package route

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

type introductionStalledWrite struct {
	net.Conn
	entered chan struct{}
	once    sync.Once
}

func (connection *introductionStalledWrite) Write(body []byte) (int, error) {
	connection.once.Do(func() { close(connection.entered) })
	return connection.Conn.Write(body)
}

func TestClosedIntroductionDeliveryResultWriteCancels(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	held := &introductionStalledWrite{Conn: local, entered: make(chan struct{})}
	owner := &ClosedIntroductionRegistration{connection: held, writer: make(chan struct{}, 1), done: make(chan struct{})}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	body, err := EncodeClosedDescriptorResult([32]byte{1}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		result <- owner.writeFrame(ctx, ClosedLaneFrame{Kind: closedFrameResult, Lane: 2, Body: body}, time.Now().Add(10*time.Second))
	}()
	select {
	case <-held.entered:
	case <-time.After(time.Second):
		t.Fatal("RESULT write did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("lost cancellation: %v", err)
		}
	case <-time.After(time.Second):
		local.Close()
		<-result
		t.Fatal("cancelled RESULT waited for peer or capsule expiry")
	}
	if len(owner.writer) != 0 {
		t.Fatal("cancelled write retained writer")
	}
}

func TestClosedIntroductionDeliveryRejectsForeignReusedAndOverBudgetChildren(t *testing.T) {
	newOwner := func() *ClosedIntroductionRegistration {
		return &ClosedIntroductionRegistration{request: ClosedRegistrationRequest{Slot: [32]byte{1}, Revision: 1, Expiry: time.Now().Add(time.Minute)},
			deliveries: make(chan *ClosedIntroductionDelivery, 16), pending: make(map[uint32]*ClosedIntroductionDelivery)}
	}
	operation := func(slot [32]byte, revision uint64, end time.Time) []byte {
		t.Helper()
		body, err := EncodeClosedIntroductionSubmission([32]byte{3}, ClosedIntroductionCapsule{Slot: slot, Revision: revision,
			Expiry: end, DeliveryNonce: [32]byte{2}, Encapsulation: [32]byte{4}, Ciphertext: make([]byte, 360)})
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	end := time.Now().UTC().Add(8 * time.Second).Truncate(time.Second)
	valid := operation([32]byte{1}, 1, end)
	for _, frame := range []ClosedLaneFrame{
		{Kind: closedFrameOperation, Lane: 1, Body: valid},
		{Kind: closedFrameOperation, Lane: 2, Body: operation([32]byte{9}, 1, end)},
		{Kind: closedFrameOperation, Lane: 2, Body: operation([32]byte{1}, 2, end)},
		{Kind: closedFrameClose, Lane: 2, Body: []byte{0}},
	} {
		if err := newOwner().receiveDelivery(frame); err == nil {
			t.Fatalf("invalid child accepted: kind=%d lane=%d", frame.Kind, frame.Lane)
		}
	}
	owner := newOwner()
	for lane := uint32(2); lane <= 32; lane += 2 {
		// Independently test queue capacity; rate is exercised below.
		owner.openings = [4]time.Time{}
		if err := owner.receiveDelivery(ClosedLaneFrame{Kind: closedFrameOperation, Lane: lane, Body: valid}); err != nil {
			t.Fatal(err)
		}
	}
	owner.openings = [4]time.Time{}
	if err := owner.receiveDelivery(ClosedLaneFrame{Kind: closedFrameOperation, Lane: 34, Body: valid}); err == nil {
		t.Fatal("17th pending delivery accepted")
	}
	owner = newOwner()
	owner.used = 1<<20 - closedIntroductionDeliveryCost
	if err := owner.receiveDelivery(ClosedLaneFrame{Kind: closedFrameOperation, Lane: 2, Body: valid}); err != nil {
		t.Fatal(err)
	}
	for _, lane := range []uint32{2, 4} {
		if err := owner.receiveDelivery(ClosedLaneFrame{Kind: closedFrameOperation, Lane: lane, Body: valid}); err == nil {
			t.Fatal("reused lane or exhausted whole-registration budget accepted")
		}
	}
	owner = newOwner()
	for lane := uint32(2); lane <= 8; lane += 2 {
		if err := owner.receiveDelivery(ClosedLaneFrame{Kind: closedFrameOperation, Lane: lane, Body: valid}); err != nil {
			t.Fatal(err)
		}
	}
	if err := owner.receiveDelivery(ClosedLaneFrame{Kind: closedFrameOperation, Lane: 10, Body: valid}); err == nil {
		t.Fatal("fifth delivery in one second accepted")
	}
}
