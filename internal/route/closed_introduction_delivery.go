//go:build linux

package route

import (
	"context"
	"errors"
	"time"
)

const closedIntroductionDeliveryCost = uint64(16 + 4096 + 16 + 16384 + 16 + 1)
const closedIntroductionWithdrawCost = uint64(16 + 4096 + 16 + 16384)

// ClosedIntroductionDelivery is one capsule on an already authorized
// registration. Completion acknowledges only this delivery, not a data join.
type ClosedIntroductionDelivery struct {
	owner             *ClosedIntroductionRegistration
	lane              uint32
	nonce             [32]byte
	operation         []byte
	end               time.Time
	done              chan struct{}
	claimed, answered bool
	status            uint8
	outcome           error
}

// Operation returns a copy of the fixed confidential capsule operation.
func (delivery *ClosedIntroductionDelivery) Operation() []byte {
	if delivery == nil || delivery.owner == nil {
		return nil
	}
	delivery.owner.mu.Lock()
	defer delivery.owner.mu.Unlock()
	if !delivery.claimed || delivery.answered {
		return nil
	}
	return append([]byte(nil), delivery.operation...)
}

// DeliveryAvailable signals queued work without transferring a capsule. This
// lets Endpoint select its current and bounded predecessor registrations.
func (owner *ClosedIntroductionRegistration) DeliveryAvailable() <-chan struct{} {
	if owner == nil {
		return nil
	}
	return owner.available
}

// TakeDelivery claims at most one current capsule. Cancellation cannot dequeue
// and silently lose another consumer's still-live delivery.
func (owner *ClosedIntroductionRegistration) TakeDelivery(ctx context.Context) (*ClosedIntroductionDelivery, error) {
	if owner == nil || ctx == nil {
		return nil, errors.New("closed Introduction delivery caller unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	select {
	case <-owner.done:
		return nil, errors.New("closed Introduction registration ended")
	default:
	}
	for {
		select {
		case delivery := <-owner.deliveries:
			if owner.pending[delivery.lane] == delivery && owner.withdraw == [32]byte{} && time.Now().Before(delivery.end) {
				delivery.claimed = true
				if len(owner.deliveries) > 0 {
					owner.signalDeliveryLocked()
				}
				return delivery, nil
			}
		default:
			return nil, nil
		}
	}
}
func (owner *ClosedIntroductionRegistration) signalDeliveryLocked() {
	select {
	case owner.available <- struct{}{}:
	default:
	}
}

// Complete sends the matching fixed RESULT and joins its terminal CLOSE before
// releasing the pending capsule. A second answer or expired owner is refused.
func (delivery *ClosedIntroductionDelivery) Complete(ctx context.Context, status uint8) error {
	if delivery == nil || delivery.owner == nil || ctx == nil || ctx.Err() != nil || status > 4 {
		return errors.New("closed Introduction delivery completion unavailable")
	}
	owner := delivery.owner
	owner.mu.Lock()
	if owner.pending[delivery.lane] != delivery || !delivery.claimed || delivery.answered || !time.Now().Before(delivery.end) {
		owner.mu.Unlock()
		return errors.New("closed Introduction delivery already ended")
	}
	delivery.answered, delivery.status = true, status
	clear(delivery.operation)
	delivery.operation = nil
	owner.mu.Unlock()
	body, err := EncodeClosedDescriptorResult(delivery.nonce, status, nil)
	if err != nil {
		return err
	}
	bounded, cancel := context.WithDeadline(ctx, delivery.end)
	defer cancel()
	if err := owner.writeFrame(bounded, ClosedLaneFrame{Kind: closedFrameResult, Lane: delivery.lane, Body: body}, delivery.end); err != nil {
		return errors.Join(err, owner.Close())
	}
	select {
	case <-delivery.done:
		owner.mu.Lock()
		err = delivery.outcome
		owner.mu.Unlock()
		return err
	case <-bounded.Done():
		return errors.Join(bounded.Err(), owner.Close())
	}
}

func (owner *ClosedIntroductionRegistration) writeFrame(ctx context.Context, frame ClosedLaneFrame, end time.Time) (outcome error) {
	bounded, cancel := context.WithDeadline(ctx, end)
	defer cancel()
	select {
	case owner.writer <- struct{}{}:
	case <-bounded.Done():
		return bounded.Err()
	case <-owner.done:
		return errors.New("closed Introduction registration ended")
	}
	defer func() { <-owner.writer }()
	if bounded.Err() != nil {
		return bounded.Err()
	}
	if err := owner.connection.SetWriteDeadline(end); err != nil {
		return err
	}
	interrupted := make(chan struct{})
	var interruptErr error
	stop := context.AfterFunc(bounded, func() {
		defer close(interrupted)
		interruptErr = owner.connection.SetWriteDeadline(time.Now())
	})
	defer func() {
		if !stop() {
			<-interrupted
		}
		outcome = errors.Join(outcome, bounded.Err(), interruptErr)
	}()
	return WriteClosedLaneFrame(owner.connection, frame)
}

func (owner *ClosedIntroductionRegistration) receiveDelivery(frame ClosedLaneFrame) error {
	// Called only by the joined reader with owner.mu held.
	if frame.Kind == closedFrameClose {
		delivery := owner.pending[frame.Lane]
		if delivery == nil || !delivery.answered || frame.Body[0] != delivery.status {
			return errors.New("closed Introduction delivery CLOSE invalid")
		}
		if !time.Now().Before(delivery.end) {
			delivery.outcome = errors.New("closed Introduction delivery expired")
		}
		delete(owner.pending, frame.Lane)
		close(delivery.done)
		return nil
	}
	if frame.Kind != closedFrameOperation || frame.Lane == 0 || frame.Lane%2 != 0 || frame.Lane <= owner.lastDelivery ||
		owner.withdraw != [32]byte{} || len(owner.pending) >= 16 || owner.used+closedIntroductionDeliveryCost > 1<<20 {
		return errors.New("closed Introduction delivery lane or budget invalid")
	}
	nonce, capsule, err := DecodeClosedIntroductionSubmission(frame.Body)
	if err != nil {
		return err
	}
	defer clear(capsule.Ciphertext)
	now := time.Now().UTC()
	if capsule.Slot != owner.request.Slot || capsule.Revision != owner.request.Revision ||
		!now.Before(capsule.Expiry) || capsule.Expiry.After(owner.request.Expiry) || capsule.Expiry.After(now.Add(10*time.Second)) ||
		now.Before(owner.openings[3]) || now.Before(owner.openings[0].Add(time.Second)) {
		return errors.New("closed Introduction delivery registration or rate invalid")
	}
	copy(owner.openings[:3], owner.openings[1:])
	owner.openings[3] = now
	owner.used += closedIntroductionDeliveryCost
	owner.lastDelivery = frame.Lane
	delivery := &ClosedIntroductionDelivery{owner: owner, lane: frame.Lane, nonce: nonce, operation: frame.Body, end: capsule.Expiry, done: make(chan struct{})}
	select {
	case owner.deliveries <- delivery:
		owner.pending[frame.Lane] = delivery
		owner.signalDeliveryLocked()
	default:
		return errors.New("closed Introduction pending delivery capacity exhausted")
	}
	return nil
}
