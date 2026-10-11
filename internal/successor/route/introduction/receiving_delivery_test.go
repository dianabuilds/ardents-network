package introduction

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// These private state/pipe controls isolate accounting and physical join. They
// grant no durable Registry claim, admitted channel or recipient authority.
func receivingDispatchState(t *testing.T, conn net.Conn) *registrationDispatch {
	t.Helper()
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(time.Minute))
	registry := &Registry{slots: make(map[[32]byte]*Registration)}
	registration := &Registration{registry: registry, acknowledged: true,
		request: Request{Nonce: [32]byte{6}, Slot: [32]byte{1}, Revision: 7, Expiry: time.Now().Add(time.Minute)},
		used:    registrationExchangeBytes, maximum: 1 << 20}
	registry.slots[registration.request.Slot] = registration
	d := &registrationDispatch{registration: registration, conn: conn, ctx: ctx, cancel: cancel,
		check: ctx.Err, record: func(error) {}, writer: make(chan struct{}, 1),
		interrupt: func() error { return conn.SetDeadline(time.Now()) },
		pending:   make(map[uint32]*receivingDelivery), nonces: make(map[[32]byte]bool)}
	registration.dispatch = d
	t.Cleanup(cancel)
	return d
}

func TestReceivingDeliveryReservesWriterWaitersWithoutByteRefund(t *testing.T) {
	d := receivingDispatchState(t, nil)
	envelope, now := deliveryEnvelope(t), time.Now()
	for i := range 16 {
		if err := d.reserve(t.Context(), envelope, [32]byte{byte(100 + i)}, now.Add(time.Duration(i/4)*time.Second)); err != nil {
			t.Fatal("bounded waiter could not reserve", i, err)
		}
	}
	used := d.registration.used
	if d.inFlight != 16 || used != 41024+16*20529 {
		t.Fatal("writer waiters escaped complete original accounting", d.inFlight, used)
	}
	if err := d.reserve(t.Context(), envelope, [32]byte{200}, now.Add(4*time.Second)); err == nil || d.registration.used != used {
		t.Fatal("seventeenth waiter reserved or changed bytes", err)
	}
	d.inFlight-- // One physically joined user returns only pending capacity.
	d.users.Done()
	if err := d.reserve(t.Context(), envelope, [32]byte{100}, now.Add(4*time.Second)); err == nil {
		t.Fatal("joined child replay nonce forgotten")
	}
	if err := d.reserve(t.Context(), envelope, [32]byte{201}, now.Add(4*time.Second)); err != nil {
		t.Fatal("joined capacity unavailable", err)
	}
	if d.registration.used != used+20529 || len(d.nonces) != 17 {
		t.Fatal("return refunded bytes or nonce history")
	}
	for range d.inFlight {
		d.users.Done()
	}
	d.inFlight = 0
	d.users.Wait()
}

func TestReceivingDeliveryRateAndWithdrawalReserveAreIndependent(t *testing.T) {
	envelope, now := deliveryEnvelope(t), time.Now()
	rate := receivingDispatchState(t, nil)
	for i := range 4 {
		if err := rate.reserve(t.Context(), envelope, [32]byte{byte(100 + i)}, now); err != nil {
			t.Fatal(err)
		}
		rate.users.Done()
		rate.inFlight--
	}
	if err := rate.reserve(t.Context(), envelope, [32]byte{105}, now.Add(999*time.Millisecond)); err == nil {
		t.Fatal("fifth rolling-second admission accepted")
	}
	if err := rate.reserve(t.Context(), envelope, [32]byte{106}, now.Add(time.Second)); err != nil {
		t.Fatal("exact rolling-second edge unavailable", err)
	}
	rate.users.Done()
	rate.inFlight--
	budget := receivingDispatchState(t, nil)
	budget.registration.maximum = 41024 + 20529
	if err := budget.reserve(t.Context(), envelope, [32]byte{110}, now); err != nil {
		t.Fatal(err)
	}
	budget.users.Done()
	budget.inFlight--
	if err := budget.reserve(t.Context(), envelope, [32]byte{111}, now.Add(time.Second)); err == nil || budget.registration.used != 61553 {
		t.Fatal("delivery borrowed withdrawal reserve or refunded bytes", err)
	}
}

type heldDeliveryClose struct {
	net.Conn
	entered, joined chan struct{}
	failure         error
	once            sync.Once
}

func (c *heldDeliveryClose) Write(raw []byte) (int, error) {
	n, err := c.Conn.Write(raw)
	if len(raw) == 17 && raw[6] == ardp.KindClose && err == nil {
		c.once.Do(func() { close(c.entered) })
		<-c.joined
		return n, c.failure
	}
	return n, err
}

func TestReceivingDeliveryRetainsLateCloseFailureUntilWriterJoins(t *testing.T) {
	for _, trial := range []struct {
		name   string
		status uint8
	}{{"accepted", 0}, {"refused", 1}} {
		t.Run(trial.name, func(t *testing.T) {
			receivingDeliveryLateCloseFailure(t, trial.status)
		})
	}
}

func receivingDeliveryLateCloseFailure(t *testing.T, status uint8) {
	holder, peer := deliveryTransportFixture(t)
	failure := errors.New("original CLOSE physical return failure")
	physical := &heldDeliveryClose{Conn: peer, entered: make(chan struct{}), joined: make(chan struct{}), failure: failure}
	d := receivingDispatchState(t, physical)
	var recorded error
	d.record = func(err error) { recorded = errors.Join(recorded, err) }
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(physical.joined) }) }
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		frame, err := ardp.ReadFrame(peer)
		if err == nil {
			err = d.receive(frame)
		}
		if err != nil {
			d.stop(err)
		}
	}()
	t.Cleanup(func() { release(); d.stop(nil); d.users.Wait(); _ = peer.Close(); <-readerDone })
	result := make(chan error, 1)
	envelope := deliveryEnvelope(t)
	go func() { _, err := d.deliver(t.Context(), envelope, [32]byte{9}); result <- err }()
	child, err := holder.NextDelivery(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	reply := make(chan error, 1)
	go func() { reply <- child.Reply(child.Context(), status) }()
	<-physical.entered
	if err := <-reply; err != nil {
		t.Fatal("matching refusal CLOSE not received", err)
	}
	child.Close()
	select {
	case err := <-result:
		t.Fatal("delivery returned before CLOSE writer joined", err)
	default:
	}
	d.stop(context.Canceled)
	joined := make(chan struct{})
	go func() { d.users.Wait(); close(joined) }()
	select {
	case <-joined:
		t.Fatal("retirement returned capacity with writer retained")
	default:
	}
	release()
	err = <-result
	if !errors.Is(err, failure) {
		t.Fatal("late physical cause lost after earlier stop", err)
	}
	<-joined
	d.registration.registry.mu.Lock()
	retained := d.failure
	used, inFlight, nonces := d.registration.used, d.inFlight, len(d.nonces)
	d.registration.registry.mu.Unlock()
	if !errors.Is(retained, failure) || !errors.Is(recorded, failure) || errors.Is(recorded, context.Canceled) || used != 61553 || inFlight != 0 || nonces != 1 {
		t.Fatal("failure/debit/nonce or physical join lost", retained, used, inFlight, nonces)
	}
}

func TestReceivingDispatchRetainsOperationCancellationWithoutReportingPhysicalFailure(t *testing.T) {
	first, second := net.Pipe()
	defer first.Close()
	defer second.Close()
	d := receivingDispatchState(t, first)
	var physical error
	d.record = func(err error) { physical = errors.Join(physical, err) }
	d.stop(context.Canceled)
	d.users.Wait()
	if !errors.Is(d.failure, context.Canceled) || !d.sealed || !d.registration.retired {
		t.Fatal("original operation lost cancellation or admission seal", d.failure)
	}
	if physical != nil {
		t.Fatal("operation cancellation manufactured a physical listener failure", physical)
	}
}
