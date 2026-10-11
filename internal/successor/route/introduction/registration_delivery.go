package introduction

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
)

// ErrDeliveryInterrupted is the original Registration's failed child outcome.
// It does not prove successful Reply, physical join or graceful retirement.
var ErrDeliveryInterrupted = errors.New("registration delivery interrupted")

// DeliveryResultFailure retains failure after this original child emitted and
// checked a successful Publisher RESULT. It proves no peer receipt, matching
// CLOSE, Connection handoff or joined cleanup. Only Reply constructs its cause.
type DeliveryResultFailure struct{ cause error }

func (err *DeliveryResultFailure) Error() string {
	if err == nil || err.cause == nil {
		return "registration delivery RESULT failure absent"
	}
	return "registration delivery failed after successful RESULT: " + err.cause.Error()
}

func (err *DeliveryResultFailure) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.cause
}

// Delivery is one original registration-local child borrow. Its sealed input
// and transport RESULT grant no Publication, Connection or replay authority.
// The consumer must Close even when the original registration is interrupted.
type Delivery struct {
	owner                           *HolderRegistration
	lane                            uint32
	nonce                           [32]byte
	envelope                        capsule.Envelope
	ctx                             context.Context
	cancel                          context.CancelFunc
	stopDeadline                    func() bool
	deadlineDone                    chan struct{}
	done, resultReady               chan struct{}
	mu                              sync.Mutex
	claimed, closed, replying, sent bool
	status                          uint8
	result                          error
	response                        sync.WaitGroup
	finishOnce, closeOnce           sync.Once
}

func (delivery *Delivery) Context() context.Context  { return delivery.ctx }
func (delivery *Delivery) Capsule() capsule.Envelope { return delivery.envelope }

// NextDelivery transfers one checked original child. Stop seals acquisition;
// claimed borrowers remain mandatory joined cleanup obligations for Close.
func (r *HolderRegistration) NextDelivery(ctx context.Context) (*Delivery, error) {
	if r == nil || ctx == nil {
		return nil, errors.New("registration delivery absent")
	}
	select {
	case delivery := <-r.deliveryQueue:
		r.mu.Lock()
		available := !r.stopped && !r.withdrawing && ctx.Err() == nil
		if available {
			delivery.claimed = true
		}
		r.mu.Unlock()
		if !available {
			delivery.Close()
			return nil, errors.New("registration delivery retired")
		}
		if err := r.CheckReceipt(r.receipt); err != nil {
			delivery.Close()
			return nil, err
		}
		return delivery, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-r.ctx.Done():
		return nil, errors.New("registration delivery retired")
	}
}

func (r *HolderRegistration) receiveDelivery(frame ardp.Frame) error {
	nonce, envelope, err := decodeDelivery(frame.Body)
	if err != nil || frame.Kind != ardp.KindOperation || frame.Lane == 0 || frame.Lane%2 != 0 {
		return errors.Join(errors.New("registration delivery lane invalid"), err)
	}
	if err := r.CheckReceipt(r.receipt); err != nil {
		return err
	}
	now, header := time.Now(), envelope.Header()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped || r.withdrawing || frame.Lane <= r.deliveryLane || len(r.deliveries) >= 16 ||
		r.deliveryNonces[nonce] || nonce == r.request.Nonce || header.Slot != r.request.Slot || header.Revision != r.request.Revision ||
		!now.Before(header.Expiry) || header.Expiry.After(r.request.Expiry) || header.Expiry.After(now.Add(10*time.Second)) ||
		r.deliveryUsed > admission.RegistrationClass.ByteLimit() || deliveryExchangeBytes > admission.RegistrationClass.ByteLimit()-r.deliveryUsed {
		return errors.New("registration delivery binding or capacity unavailable")
	}
	attempts := r.deliveryStarts[:0]
	for _, start := range r.deliveryStarts {
		if start.After(now.Add(-time.Second)) {
			attempts = append(attempts, start)
		}
	}
	r.deliveryStarts = attempts
	if len(attempts) >= 4 {
		return errors.New("registration delivery rate unavailable")
	}
	r.deliveryStarts = append(attempts, now)
	r.deliveryUsed += deliveryExchangeBytes // No refund, including failed dispatch.
	r.deliveryLane = frame.Lane
	r.deliveryNonces[nonce] = true
	ctx, cancel := context.WithDeadline(r.ctx, header.Expiry)
	delivery := &Delivery{owner: r, lane: frame.Lane, nonce: nonce, envelope: envelope, ctx: ctx, cancel: cancel,
		done: make(chan struct{}), resultReady: make(chan struct{}), deadlineDone: make(chan struct{})}
	delivery.stopDeadline = context.AfterFunc(ctx, func() { defer close(delivery.deadlineDone); r.stop(ctx.Err()) })
	r.deliveries[frame.Lane] = delivery
	r.deliveryUsers.Add(1)
	r.deliveryQueue <- delivery // Bounded by the retained 16-child map above.
	return nil
}

// Reply writes one fixed transport outcome, then waits for its exact terminal
// CLOSE. Only the Publication consumer may decide whether downstream facts
// warrant success; a status byte here does not confer that authority.
func (delivery *Delivery) Reply(ctx context.Context, status uint8) (result error) {
	if delivery == nil || ctx == nil || status > 4 {
		return errors.New("registration delivery reply invalid")
	}
	delivery.mu.Lock()
	if delivery.closed || delivery.replying || delivery.ctx.Err() != nil {
		delivery.mu.Unlock()
		return errors.New("registration delivery reply unavailable")
	}
	delivery.replying, delivery.status = true, status
	delivery.response.Add(1)
	delivery.mu.Unlock()
	defer delivery.response.Done()
	r := delivery.owner
	callerDone := make(chan struct{})
	stopCaller := context.AfterFunc(ctx, func() { defer close(callerDone); r.stop(ctx.Err()) })
	defer func() {
		if !stopCaller() {
			<-callerDone
		}
		result = errors.Join(result, ctx.Err())
		if result != nil {
			delivery.mu.Lock()
			successfulResult := delivery.sent && delivery.status == 0
			delivery.mu.Unlock()
			if successfulResult {
				result = &DeliveryResultFailure{cause: result}
			}
			r.mu.Lock()
			r.deliveryErr = errors.Join(r.deliveryErr, result)
			r.mu.Unlock()
		}
	}()
	body, err := EncodeResult(delivery.nonce, status)
	r.writer.Lock()
	if err == nil {
		err = errors.Join(ctx.Err(), delivery.ctx.Err(), r.CheckReceipt(r.receipt))
	}
	if err == nil {
		err = ardp.WriteFrame(r.conn, ardp.Frame{Kind: ardp.KindResult, Lane: delivery.lane, Body: body})
	}
	if err == nil {
		err = errors.Join(ctx.Err(), delivery.ctx.Err(), r.CheckReceipt(r.receipt))
	}
	r.writer.Unlock()
	delivery.mu.Lock()
	delivery.sent = err == nil
	delivery.mu.Unlock()
	close(delivery.resultReady)
	if err != nil {
		r.stop(err)
		return err
	}
	<-delivery.done
	delivery.mu.Lock()
	terminalResult := delivery.result
	delivery.mu.Unlock()
	if terminalResult != nil {
		return terminalResult
	}
	return errors.Join(ctx.Err(), delivery.ctx.Err(), r.CheckReceipt(r.receipt))
}

func (r *HolderRegistration) closeDelivery(frame ardp.Frame) error {
	r.mu.Lock()
	delivery := r.deliveries[frame.Lane]
	r.mu.Unlock()
	if delivery == nil || frame.Kind != ardp.KindClose || len(frame.Body) != 1 {
		return errors.New("registration delivery CLOSE absent")
	}
	select {
	case <-delivery.done:
		return errors.New("registration delivery CLOSE repeated")
	default:
	}
	delivery.mu.Lock()
	replying := delivery.replying
	delivery.mu.Unlock()
	if !replying {
		return errors.New("registration delivery CLOSE before RESULT")
	}
	select {
	case <-delivery.resultReady:
	case <-r.ctx.Done():
		return r.ctx.Err()
	}
	delivery.mu.Lock()
	valid := delivery.sent && frame.Body[0] == delivery.status && delivery.ctx.Err() == nil
	delivery.mu.Unlock()
	if !valid {
		return errors.New("registration delivery CLOSE differs")
	}
	if err := r.CheckReceipt(r.receipt); err != nil {
		return err
	}
	delivery.finish(nil)
	return nil
}

func (delivery *Delivery) finish(result error) {
	delivery.finishOnce.Do(func() {
		if !delivery.stopDeadline() {
			<-delivery.deadlineDone
		}
		delivery.mu.Lock()
		delivery.result = result
		delivery.mu.Unlock()
		close(delivery.done)
	})
}

func (r *HolderRegistration) abortDeliveries() {
	r.mu.Lock()
	deliveries := make([]*Delivery, 0, len(r.deliveries))
	for _, delivery := range r.deliveries {
		deliveries = append(deliveries, delivery)
	}
	r.mu.Unlock()
	for _, delivery := range deliveries {
		delivery.finish(ErrDeliveryInterrupted)
	}
}

func (delivery *Delivery) Close() {
	if delivery == nil {
		return
	}
	delivery.closeOnce.Do(func() {
		delivery.mu.Lock()
		delivery.closed = true
		delivery.mu.Unlock()
		select {
		case <-delivery.done:
		default:
			delivery.owner.stop(errors.New("registration delivery abandoned"))
		}
		delivery.response.Wait()
		<-delivery.done
		delivery.cancel()
		r := delivery.owner
		r.mu.Lock()
		delete(r.deliveries, delivery.lane)
		r.mu.Unlock()
		r.deliveryUsers.Done()
	})
}
