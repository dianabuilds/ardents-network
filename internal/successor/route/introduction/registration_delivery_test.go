package introduction

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// This fixture isolates framing and joined child ownership over net.Pipe.
// Its checker observes local cancellation only; it grants no genuine Prefix,
// REGISTER, worker, Publication, capsule authentication or Connection authority.
func deliveryTransportFixture(t *testing.T) (*HolderRegistration, net.Conn) {
	t.Helper()
	conn, peer := net.Pipe()
	ctx, cancel := context.WithCancel(t.Context())
	r := &HolderRegistration{conn: conn, ctx: ctx, caller: ctx, cancel: cancel, check: ctx.Err,
		request:    Request{Nonce: [32]byte{6}, Slot: [32]byte{1}, Revision: 7, Expiry: time.Now().Add(time.Minute)},
		readerDone: make(chan struct{}), retiring: make(chan struct{}), reply: make(chan error, 1),
		deliveries: make(map[uint32]*Delivery), deliveryNonces: make(map[[32]byte]bool), deliveryQueue: make(chan *Delivery, 16),
		deliveryUsed: registrationExchangeBytes}
	r.receipt = Receipt{owner: r, facts: RegistrationFacts{Slot: r.request.Slot, Revision: 7, Expiry: r.request.Expiry}}
	physicalDone := make(chan struct{})
	context.AfterFunc(ctx, func() { defer close(physicalDone); _ = conn.Close() })
	go r.read()
	t.Cleanup(func() {
		r.stop(nil)
		_ = peer.Close()
		<-r.readerDone
		<-physicalDone
		r.mu.Lock()
		pending := make([]*Delivery, 0, len(r.deliveries))
		for _, child := range r.deliveries {
			pending = append(pending, child)
		}
		r.mu.Unlock()
		for _, child := range pending {
			child.Close()
		}
		r.deliveryUsers.Wait()
	})
	return r, peer
}

func TestDeliveryTransportJoinsMatchingTerminalBeforeBorrowReturn(t *testing.T) {
	r, peer := deliveryTransportFixture(t)
	body, err := EncodeDelivery([32]byte{9}, deliveryEnvelope(t))
	if err != nil {
		t.Fatal(err)
	}
	written := make(chan error, 1)
	go func() { written <- ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindOperation, Lane: 2, Body: body}) }()
	delivery, err := r.NextDelivery(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	replied := make(chan error, 1)
	go func() { replied <- delivery.Reply(delivery.Context(), 1) }()
	frame, err := ardp.ReadFrame(peer)
	if err != nil {
		t.Fatal(err)
	}
	if status, err := DecodeResult(frame.Body, [32]byte{9}); err != nil || status != 1 || frame.Kind != ardp.KindResult || frame.Lane != 2 {
		t.Fatal("matching fixed refusal RESULT differs", err)
	}
	select {
	case err := <-replied:
		t.Fatal("reply returned before terminal CLOSE", err)
	default:
	}
	if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindClose, Lane: 2, Body: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	if err := <-replied; err != nil {
		t.Fatal("successful terminal canceled its original caller", err)
	}
	r.mu.Lock()
	retained, used := r.deliveries[2] == delivery, r.deliveryUsed
	r.mu.Unlock()
	if !retained {
		t.Fatal("terminal released consumer borrow")
	}
	delivery.Close()
	delivery.Close()
	r.deliveryUsers.Wait()
	if r.deliveryUsed != used || used != registrationExchangeBytes+deliveryExchangeBytes {
		t.Fatal("borrow return refunded original allowance")
	}
	if r.ctx.Err() != nil {
		t.Fatal("completed child retired its live registration")
	}
}

func TestDeliveryCallerCanceledWhileWaitingWriterCannotEmitResult(t *testing.T) {
	r, peer := deliveryTransportFixture(t)
	body, err := EncodeDelivery([32]byte{9}, deliveryEnvelope(t))
	if err != nil {
		t.Fatal(err)
	}
	written := make(chan error, 1)
	go func() { written <- ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindOperation, Lane: 2, Body: body}) }()
	delivery, err := r.NextDelivery(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	r.writer.Lock()
	var unlockOnce sync.Once
	unlock := func() { unlockOnce.Do(r.writer.Unlock) }
	t.Cleanup(unlock)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	replied := make(chan error, 1)
	go func() { replied <- delivery.Reply(ctx, 1) }()
	<-r.ctx.Done()
	unlock()
	if err := <-replied; !errors.Is(err, context.Canceled) {
		t.Fatal("original canceled caller result erased", err)
	}
	if _, err := ardp.ReadFrame(peer); err == nil {
		t.Fatal("canceled writer emitted RESULT")
	}
	delivery.Close()
}

func TestDeliveryRetainsEmittedSuccessfulResultWhenTerminalFails(t *testing.T) {
	for _, trial := range []struct {
		name   string
		status uint8
	}{{"accepted", 0}, {"refused", 1}} {
		t.Run(trial.name, func(t *testing.T) {
			r, peer := deliveryTransportFixture(t)
			body, err := EncodeDelivery([32]byte{9}, deliveryEnvelope(t))
			if err != nil {
				t.Fatal(err)
			}
			written := make(chan error, 1)
			go func() { written <- ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindOperation, Lane: 2, Body: body}) }()
			delivery, err := r.NextDelivery(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if err := <-written; err != nil {
				t.Fatal(err)
			}
			caller, cancel := context.WithCancel(t.Context())
			defer cancel()
			replied := make(chan error, 1)
			go func() { replied <- delivery.Reply(caller, trial.status) }()
			frame, err := ardp.ReadFrame(peer)
			if err != nil || frame.Kind != ardp.KindResult || frame.Lane != 2 {
				t.Fatal("original RESULT absent", err)
			}
			if status, err := DecodeResult(frame.Body, [32]byte{9}); err != nil || status != trial.status {
				t.Fatal("actual matching RESULT differs", status, err)
			}
			<-delivery.resultReady
			cancel() // No matching CLOSE is sent by the independent peer.
			err = <-replied
			var emitted *DeliveryResultFailure
			if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrDeliveryInterrupted) || errors.As(err, &emitted) != (trial.status == 0) || emitted != nil && emitted.Unwrap() == nil {
				t.Fatal("failed terminal lost original cause or actual RESULT provenance", err)
			}
			r.mu.Lock()
			retained := r.deliveryErr
			r.mu.Unlock()
			var original *DeliveryResultFailure
			if errors.As(retained, &original) != (trial.status == 0) || original != emitted {
				t.Fatal("Registration failed result lost exact child provenance", retained)
			}
			delivery.Close()
			r.deliveryUsers.Wait()
		})
	}
}

func TestDeliveryAccountingReservesWholeExchangeAndRetainsPendingLimit(t *testing.T) {
	r, _ := deliveryTransportFixture(t)
	envelope := deliveryEnvelope(t)
	for number := byte(1); number <= 4; number++ {
		body, err := EncodeDelivery([32]byte{number}, envelope)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.receiveDelivery(ardp.Frame{Kind: ardp.KindOperation, Lane: uint32(number) * 2, Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	body, err := EncodeDelivery([32]byte{10}, envelope)
	if err != nil {
		t.Fatal(err)
	}
	if r.receiveDelivery(ardp.Frame{Kind: ardp.KindOperation, Lane: 10, Body: body}) == nil {
		t.Fatal("fifth rolling-second start accepted")
	}
	if r.deliveryUsed != registrationExchangeBytes+4*deliveryExchangeBytes {
		t.Fatal("complete operation/result/close reserve changed")
	}
	for number := byte(11); number <= 22; number++ {
		r.mu.Lock()
		r.deliveryStarts = nil // Isolate the 16-child capacity from elapsed-rate qualification.
		r.mu.Unlock()
		body, err := EncodeDelivery([32]byte{number}, envelope)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.receiveDelivery(ardp.Frame{Kind: ardp.KindOperation, Lane: uint32(number) * 2, Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	if len(r.deliveries) != 16 {
		t.Fatal("pending child accounting differs")
	}
	r.mu.Lock()
	r.deliveryStarts = nil
	r.mu.Unlock()
	if r.receiveDelivery(ardp.Frame{Kind: ardp.KindOperation, Lane: 46, Body: body}) == nil {
		t.Fatal("seventeenth pending child accepted")
	}
	budgetOwner, _ := deliveryTransportFixture(t)
	budgetOwner.deliveryUsed = admission.RegistrationClass.ByteLimit() - deliveryExchangeBytes + 1
	if budgetOwner.receiveDelivery(ardp.Frame{Kind: ardp.KindOperation, Lane: 2, Body: body}) == nil {
		t.Fatal("partial RESULT/CLOSE allowance accepted")
	}
}

func TestDeliveryDuplicateTerminalRetiresOriginalRegistration(t *testing.T) {
	r, peer := deliveryTransportFixture(t)
	body, err := EncodeDelivery([32]byte{9}, deliveryEnvelope(t))
	if err != nil {
		t.Fatal(err)
	}
	written := make(chan error, 1)
	go func() { written <- ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindOperation, Lane: 2, Body: body}) }()
	delivery, err := r.NextDelivery(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	replied := make(chan error, 1)
	go func() { replied <- delivery.Reply(delivery.Context(), 1) }()
	if _, err := ardp.ReadFrame(peer); err != nil {
		t.Fatal(err)
	}
	terminal := ardp.Frame{Kind: ardp.KindClose, Lane: 2, Body: []byte{1}}
	if err := ardp.WriteFrame(peer, terminal); err != nil {
		t.Fatal(err)
	}
	if err := <-replied; err != nil {
		t.Fatal(err)
	}
	if err := ardp.WriteFrame(peer, terminal); err != nil {
		t.Fatal(err)
	}
	<-r.ctx.Done()
	if r.failure == nil {
		t.Fatal("duplicate CLOSE retained accepting registration")
	}
	delivery.Close()
}
