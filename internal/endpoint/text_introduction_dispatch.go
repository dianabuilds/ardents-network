//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
)

const maximumTextIntroductionWaiters = 16

type textIntroductionDeliveryKey struct {
	connection [32]byte
	generation uint64
}

type textIntroductionWaiter struct {
	want     textIntroductionDeliveryKey
	delivery chan *route.ClosedIntroductionDelivery
}

// dispatchTextIntroductionDelivery registers the exact local owner before it
// competes for the one consumer gate. The consumer routes each claimed capsule
// directly to an already registered waiter; unmatched inputs are refused while
// completion is still possible and never occupy registration capacity.
func (owner *textContext) dispatchTextIntroductionDelivery(ctx context.Context, job *textJobIdentity,
	want textIntroductionDeliveryKey) (delivery *route.ClosedIntroductionDelivery, outcome error) {
	waiter, gate, err := owner.registerTextIntroductionWaiter(ctx, job, want)
	if err != nil {
		return nil, err
	}
	defer func() {
		outcome = errors.Join(outcome, owner.releaseTextIntroductionWaiter(waiter))
		if outcome != nil {
			delivery = nil
		}
	}()
	for {
		select {
		case delivery = <-waiter.delivery:
			return delivery, nil
		default:
		}
		select {
		case delivery = <-waiter.delivery:
			return delivery, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-gate:
		}
		// Another consumer may have assigned this waiter immediately before it
		// released the gate. Do not block the assigned owner behind a fresh take.
		select {
		case delivery = <-waiter.delivery:
			gate <- struct{}{}
			return delivery, nil
		default:
		}
		for {
			claimed, key, err := owner.claimTextIntroductionDelivery(ctx, job)
			if err != nil {
				gate <- struct{}{}
				return nil, err
			}
			owner.mu.Lock()
			target := owner.selectTextIntroductionWaiterLocked(key)
			if target != nil && target != waiter {
				target.delivery <- claimed
			}
			owner.mu.Unlock()
			if target == waiter {
				gate <- struct{}{}
				return claimed, nil
			}
			if target != nil {
				continue
			}
			// No local recovery owns this identity. Refuse it now instead of
			// retaining a claimed lane until Complete can no longer answer it.
			gate <- struct{}{}
			if err := claimed.Complete(ctx, 1); err != nil {
				return nil, err
			}
			break
		}
	}
}

func (owner *textContext) registerTextIntroductionWaiter(ctx context.Context, job *textJobIdentity,
	want textIntroductionDeliveryKey) (*textIntroductionWaiter, chan struct{}, error) {
	if owner == nil || ctx == nil || want.generation == 0 {
		return nil, nil, errors.New("text Introduction dispatch unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if !owner.liveTextServiceJobLocked(job, broker.Administration) || ctx.Err() != nil {
		return nil, nil, errors.Join(ctx.Err(), errors.New("text Introduction dispatch owner retired"))
	}
	if len(owner.introductionWaiters) >= maximumTextIntroductionWaiters {
		return nil, nil, errors.New("text Introduction dispatch capacity unavailable")
	}
	if owner.introductionDelivery == nil {
		owner.introductionDelivery = make(chan struct{}, 1)
		owner.introductionDelivery <- struct{}{}
	}
	if owner.introductionWaiters == nil {
		owner.introductionWaiters = make(map[*textIntroductionWaiter]struct{})
	}
	waiter := &textIntroductionWaiter{want: want, delivery: make(chan *route.ClosedIntroductionDelivery, 1)}
	owner.introductionWaiters[waiter] = struct{}{}
	return waiter, owner.introductionDelivery, nil
}

// releaseTextIntroductionWaiter joins ownership of a delivery assigned at the
// same instant its waiter was canceled. A live context refuses that capsule;
// terminal context cleanup instead closes the whole registration owner.
func (owner *textContext) releaseTextIntroductionWaiter(waiter *textIntroductionWaiter) error {
	if owner == nil || waiter == nil {
		return nil
	}
	owner.mu.Lock()
	delete(owner.introductionWaiters, waiter)
	var delivery *route.ClosedIntroductionDelivery
	select {
	case delivery = <-waiter.delivery:
	default:
	}
	lifetime := owner.lease.Context()
	owner.mu.Unlock()
	if delivery == nil || lifetime.Err() != nil {
		return nil
	}
	return delivery.Complete(lifetime, 1)
}

func (owner *textContext) selectTextIntroductionWaiterLocked(key textIntroductionDeliveryKey) *textIntroductionWaiter {
	for waiter := range owner.introductionWaiters {
		if len(waiter.delivery) == 0 && textIntroductionDeliveryMatches(key, waiter.want) {
			return waiter
		}
	}
	return nil
}

func (owner *textContext) claimTextIntroductionDelivery(ctx context.Context,
	job *textJobIdentity) (*route.ClosedIntroductionDelivery, textIntroductionDeliveryKey, error) {
	delivery, err := owner.nextTextIntroductionDelivery(ctx)
	if err != nil {
		return nil, textIntroductionDeliveryKey{}, err
	}
	key, err := owner.inspectTextIntroductionDelivery(ctx, job, delivery)
	if err != nil {
		completeErr := delivery.Complete(ctx, 1)
		return nil, textIntroductionDeliveryKey{}, errors.Join(err, completeErr)
	}
	return delivery, key, nil
}

// inspectTextIntroductionDelivery decrypts only enough capsule state to route
// ownership. It consumes the existing cryptographic-opening rate reservation
// before decryption; acceptance repeats every other authority, publication,
// replay and bound check before acknowledging success.
func (owner *textContext) inspectTextIntroductionDelivery(ctx context.Context, job *textJobIdentity,
	delivery *route.ClosedIntroductionDelivery) (textIntroductionDeliveryKey, error) {
	operation := delivery.Operation()
	defer clear(operation)
	_, capsule, err := route.DecodeClosedIntroductionSubmission(operation)
	if err != nil {
		return textIntroductionDeliveryKey{}, &textIntroductionRefusal{cause: err}
	}
	defer clear(capsule.Ciphertext)
	endpoint := owner.endpoint
	endpoint.publisherMu.Lock()
	defer endpoint.publisherMu.Unlock()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.textPermissionProfileLocked()
	registered := owner.registration
	if prior := owner.previousRegistration; prior != nil && now.Before(owner.previousUntil) &&
		prior.request.Slot == capsule.Slot && prior.request.Revision == capsule.Revision {
		registered = prior
	}
	if err != nil || ctx.Err() != nil || !owner.liveTextServiceJobLocked(job, broker.Administration) ||
		registered == nil || owner.withdrawal != nil || endpoint.textPublisherOwner != owner ||
		!endpoint.textPublicationLive || endpoint.publisherBinding == nil || endpoint.publications == nil ||
		!registered.published || registered.recipient == nil ||
		capsule.Slot != registered.request.Slot || capsule.Revision != registered.request.Revision ||
		!now.Before(capsule.Expiry) || capsule.Expiry.After(registered.request.Expiry) {
		return textIntroductionDeliveryKey{}, &textIntroductionRefusal{cause: errors.New("text Introduction dispatch input unavailable")}
	}
	select {
	case <-registered.channel.Done():
		return textIntroductionDeliveryKey{}, errors.New("text Introduction registration ended")
	default:
	}
	if err := owner.reserveTextIntroductionOpeningLocked(capsule.DeliveryNonce, now); err != nil {
		return textIntroductionDeliveryKey{}, &textIntroductionRefusal{cause: err}
	}
	plaintext, _, err := route.OpenClosedIntroduction(capsule, profile.Digest, registered.recipient, now)
	if err != nil {
		return textIntroductionDeliveryKey{}, &textIntroductionRefusal{cause: err}
	}
	key := textIntroductionDeliveryKey{connection: plaintext.ConnectionNonce, generation: plaintext.AttachmentGeneration}
	clear(plaintext.JoinSecret[:])
	clear(plaintext.HandshakeContext[:])
	if key.connection == [32]byte{} || key.generation == 0 {
		return textIntroductionDeliveryKey{}, &textIntroductionRefusal{cause: errors.New("text Introduction dispatch identity unavailable")}
	}
	return key, nil
}

func textIntroductionDeliveryMatches(key, want textIntroductionDeliveryKey) bool {
	return key.generation == want.generation && (want.generation == 1 || key.connection == want.connection)
}
