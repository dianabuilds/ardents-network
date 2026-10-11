package introduction

import (
	"context"
	"crypto/rand"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
)

// registrationDispatch belongs only to the original authenticated registration
// stream. Its reader is ServeRegistration; all output shares this writer. The
// receiving work owner still closes the connection and returns its reservation.
type registrationDispatch struct {
	registration *Registration
	conn         net.Conn
	ctx          context.Context
	cancel       context.CancelFunc
	check        func() error
	record       func(error)
	interrupt    func() error
	writer       chan struct{}
	users        sync.WaitGroup
	stopOnce     sync.Once
	// All following state is guarded by the original Registry mutex.
	pending    map[uint32]*receivingDelivery
	nonces     map[[32]byte]bool
	admitted   []time.Time
	dispatched []time.Time
	lastTime   time.Time
	lane       uint32
	inFlight   int
	sealed     bool
	failure    error
}

type receivingDelivery struct {
	nonce    [32]byte
	ctx      context.Context
	ready    chan uint8
	received bool
}

func (d *registrationDispatch) stop(cause error) {
	r := d.registration.registry
	r.mu.Lock()
	d.sealed = true
	d.registration.retired = true
	d.failure = errors.Join(d.failure, cause)
	r.mu.Unlock()
	d.stopOnce.Do(func() {
		d.cancel()
		err := d.interrupt()
		r.mu.Lock()
		d.failure = errors.Join(d.failure, err)
		r.mu.Unlock()
		if err != nil {
			d.record(err)
		}
	})
}

func deliveryContextCurrent(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if end, bounded := ctx.Deadline(); bounded && !time.Now().Before(end) {
		return context.DeadlineExceeded
	}
	return nil
}

func (d *registrationDispatch) write(ctx context.Context, frame ardp.Frame) error {
	select {
	case d.writer <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	case <-d.ctx.Done():
		return d.ctx.Err()
	}
	defer func() { <-d.writer }()
	if err := errors.Join(deliveryContextCurrent(ctx), deliveryContextCurrent(d.ctx), d.check()); err != nil {
		return err
	}
	if err := errors.Join(deliveryContextCurrent(ctx), deliveryContextCurrent(d.ctx)); err != nil {
		return err
	}
	if err := ardp.WriteFrame(d.conn, frame); err != nil {
		d.record(err)
		return err
	}
	return errors.Join(d.check(), deliveryContextCurrent(ctx), deliveryContextCurrent(d.ctx))
}

func retainDeliveryStarts(starts []time.Time, now time.Time) []time.Time {
	kept := starts[:0]
	for _, start := range starts {
		if start.After(now.Add(-time.Second)) {
			kept = append(kept, start)
		}
	}
	return kept
}

// reserve holds even writer waiters against the exact live original slot. The
// full exchange is irreversibly charged before output; withdrawal is separate.
func (d *registrationDispatch) reserve(ctx context.Context, envelope capsule.Envelope, nonce [32]byte, now time.Time) error {
	r := d.registration.registry
	r.mu.Lock()
	defer r.mu.Unlock()
	registration, header := d.registration, envelope.Header()
	if ctx.Err() != nil || d.ctx.Err() != nil || d.sealed || r.closed || registration.retired || registration.withdrawn ||
		!registration.acknowledged || r.slots[header.Slot] != registration || header.Revision != registration.request.Revision ||
		!now.Before(header.Expiry) || header.Expiry.After(registration.request.Expiry) || header.Expiry.After(now.Add(10*time.Second)) ||
		now.Before(d.lastTime) || nonce == [32]byte{} || nonce == registration.request.Nonce || nonce == header.DeliveryNonce || d.nonces[nonce] ||
		d.inFlight >= 16 || registration.used > registration.maximum || deliveryExchangeBytes > registration.maximum-registration.used {
		return errors.New("introduction delivery unavailable")
	}
	d.admitted = retainDeliveryStarts(d.admitted, now)
	if len(d.admitted) >= 4 {
		return errors.New("introduction delivery admission rate unavailable")
	}
	d.lastTime = now
	d.admitted = append(d.admitted, now)
	d.nonces[nonce] = true
	d.inFlight++
	registration.used += deliveryExchangeBytes
	d.users.Add(1)
	return nil
}

func (d *registrationDispatch) receive(frame ardp.Frame) error {
	r := d.registration.registry
	r.mu.Lock()
	defer r.mu.Unlock()
	pending := d.pending[frame.Lane]
	if frame.Kind != ardp.KindResult || pending == nil || pending.received || d.sealed ||
		deliveryContextCurrent(pending.ctx) != nil || deliveryContextCurrent(d.ctx) != nil || d.registration.retired {
		return errors.New("introduction delivery RESULT unavailable")
	}
	status, err := DecodeResult(frame.Body, pending.nonce)
	if err != nil {
		return err
	}
	pending.received = true
	pending.ready <- status
	return nil
}

// deliver never decodes recipient plaintext or accepts Connection facts. The
// holder's exact RESULT and completed matching CLOSE determine only transport.
func (d *registrationDispatch) deliver(caller context.Context, envelope capsule.Envelope, nonce [32]byte) (status uint8, result error) {
	ctx, cancel := context.WithDeadline(caller, envelope.Header().Expiry)
	defer cancel()
	if err := d.reserve(ctx, envelope, nonce, time.Now()); err != nil {
		return 1, err
	}
	defer d.users.Done()
	defer func() {
		r := d.registration.registry
		r.mu.Lock()
		d.inFlight-- // Nonces and irreversible byte debit remain until retirement.
		r.mu.Unlock()
	}()
	callbackDone := make(chan struct{})
	stopCaller := context.AfterFunc(ctx, func() { defer close(callbackDone); d.stop(ctx.Err()) })
	defer func() {
		if !stopCaller() {
			<-callbackDone
		}
		result = errors.Join(result, deliveryContextCurrent(ctx), deliveryContextCurrent(d.ctx))
		if result != nil {
			d.stop(result)
		}
	}()
	body, err := EncodeDelivery(nonce, envelope)
	if err != nil {
		return 1, err
	}
	select {
	case d.writer <- struct{}{}:
	case <-ctx.Done():
		return 1, ctx.Err()
	case <-d.ctx.Done():
		return 1, d.ctx.Err()
	}
	// Allocation and OPERATION output are one writer operation. Currentness
	// observation runs outside the Registry lock, then original state is checked.
	err = errors.Join(d.check(), deliveryContextCurrent(ctx), deliveryContextCurrent(d.ctx))
	r := d.registration.registry
	r.mu.Lock()
	now := time.Now()
	d.dispatched = retainDeliveryStarts(d.dispatched, now)
	if err == nil && (d.sealed || r.closed || d.registration.retired || now.Before(d.lastTime) || len(d.dispatched) >= 4 || d.lane > ^uint32(0)-2) {
		err = errors.New("introduction delivery dispatch unavailable")
	}
	var lane uint32
	pending := &receivingDelivery{nonce: nonce, ctx: ctx, ready: make(chan uint8, 1)}
	if err == nil {
		d.lastTime = now
		d.dispatched = append(d.dispatched, now)
		d.lane += 2
		lane = d.lane
		d.pending[lane] = pending
	}
	r.mu.Unlock()
	if err == nil {
		err = errors.Join(deliveryContextCurrent(ctx), deliveryContextCurrent(d.ctx))
	}
	if err == nil {
		err = ardp.WriteFrame(d.conn, ardp.Frame{Kind: ardp.KindOperation, Lane: lane, Body: body})
		if err != nil {
			d.record(err)
		}
	}
	if err == nil {
		err = errors.Join(d.check(), deliveryContextCurrent(ctx), deliveryContextCurrent(d.ctx))
	}
	<-d.writer
	defer func() { r.mu.Lock(); delete(d.pending, lane); r.mu.Unlock() }()
	if err != nil {
		return 1, err
	}
	select {
	case status = <-pending.ready:
	case <-ctx.Done():
		return 1, ctx.Err()
	case <-d.ctx.Done():
		return 1, d.ctx.Err()
	}
	if err := d.write(ctx, ardp.Frame{Kind: ardp.KindClose, Lane: lane, Body: []byte{status}}); err != nil {
		return 1, err
	}
	return status, nil
}

func (registry *Registry) deliver(ctx context.Context, sourceNonce [32]byte, envelope capsule.Envelope) (uint8, error) {
	if registry == nil || ctx == nil {
		return 1, errors.New("introduction registry unavailable")
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return 1, err
	}
	if nonce == sourceNonce {
		return 1, errors.New("introduction delivery nonce collision")
	}
	registry.mu.Lock()
	registration := registry.slots[envelope.Header().Slot]
	var dispatch *registrationDispatch
	if registration != nil {
		dispatch = registration.dispatch
	}
	registry.mu.Unlock()
	if dispatch == nil {
		return 1, errors.New("introduction delivery channel unavailable")
	}
	return dispatch.deliver(ctx, envelope, nonce)
}
