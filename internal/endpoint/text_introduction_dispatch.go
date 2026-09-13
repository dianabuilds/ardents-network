//go:build linux

package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
)

const maximumTextIntroductionPending = 16

type textIntroductionDeliveryKey struct {
	connection [32]byte
	generation uint64
}

type textIntroductionPendingDelivery struct {
	delivery *route.ClosedIntroductionDelivery
	expires  time.Time
}

// dispatchTextIntroductionDelivery is the only owner that claims a Publisher
// registration delivery. Callers may wait concurrently, but the one-token gate
// serializes the actual channel consumer and the pending map transfers a
// mismatched generation to its initial or recovery owner without completing it.
func (owner *textContext) dispatchTextIntroductionDelivery(ctx context.Context, job *textJobIdentity,
	want textIntroductionDeliveryKey) (*route.ClosedIntroductionDelivery, error) {
	if owner == nil || ctx == nil || want.generation == 0 {
		return nil, errors.New("text Introduction dispatch unavailable")
	}
	for {
		owner.mu.Lock()
		if !owner.liveTextServiceJobLocked(job, broker.Administration) || ctx.Err() != nil {
			owner.mu.Unlock()
			return nil, errors.Join(ctx.Err(), errors.New("text Introduction dispatch owner retired"))
		}
		if pending, ok := owner.takePendingTextIntroductionLocked(want, owner.endpoint.clock()); ok {
			owner.mu.Unlock()
			return pending.delivery, nil
		}
		if owner.introductionDelivery == nil {
			owner.introductionDelivery = make(chan struct{}, 1)
			owner.introductionDelivery <- struct{}{}
		}
		gate := owner.introductionDelivery
		owner.mu.Unlock()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-gate:
		}
		delivery, key, expires, err := owner.claimTextIntroductionDelivery(ctx, job, want)
		gate <- struct{}{}
		if err != nil {
			return nil, err
		}
		if textIntroductionDeliveryMatches(key, want) {
			return delivery, nil
		}
		owner.mu.Lock()
		owner.pruneTextIntroductionPendingLocked(owner.endpoint.clock())
		if owner.textIntroductionPendingCountLocked() >= maximumTextIntroductionPending {
			owner.mu.Unlock()
			completeErr := delivery.Complete(ctx, 1)
			return nil, errors.Join(&textIntroductionRefusal{cause: errors.New("text Introduction dispatch capacity unavailable")}, completeErr)
		}
		if owner.introductionPending == nil {
			owner.introductionPending = make(map[textIntroductionDeliveryKey][]textIntroductionPendingDelivery)
		}
		owner.introductionPending[key] = append(owner.introductionPending[key], textIntroductionPendingDelivery{
			delivery: delivery,
			expires:  expires,
		})
		owner.mu.Unlock()
	}
}

func (owner *textContext) claimTextIntroductionDelivery(ctx context.Context, job *textJobIdentity,
	want textIntroductionDeliveryKey) (*route.ClosedIntroductionDelivery, textIntroductionDeliveryKey, time.Time, error) {
	owner.mu.Lock()
	if pending, ok := owner.takePendingTextIntroductionLocked(want, owner.endpoint.clock()); ok {
		owner.mu.Unlock()
		return pending.delivery, want, pending.expires, nil
	}
	owner.mu.Unlock()
	delivery, err := owner.nextTextIntroductionDelivery(ctx)
	if err != nil {
		return nil, textIntroductionDeliveryKey{}, time.Time{}, err
	}
	key, expires, err := owner.inspectTextIntroductionDelivery(ctx, job, delivery)
	if err != nil {
		completeErr := delivery.Complete(ctx, 1)
		return nil, textIntroductionDeliveryKey{}, time.Time{}, errors.Join(err, completeErr)
	}
	return delivery, key, expires, nil
}

// inspectTextIntroductionDelivery decrypts only enough capsule state to route
// ownership. It consumes the existing cryptographic-opening rate reservation
// before decryption; acceptance repeats every other authority, publication,
// replay and bound check before acknowledging success.
func (owner *textContext) inspectTextIntroductionDelivery(ctx context.Context, job *textJobIdentity,
	delivery *route.ClosedIntroductionDelivery) (textIntroductionDeliveryKey, time.Time, error) {
	operation := delivery.Operation()
	defer clear(operation)
	_, capsule, err := route.DecodeClosedIntroductionSubmission(operation)
	if err != nil {
		return textIntroductionDeliveryKey{}, time.Time{}, &textIntroductionRefusal{cause: err}
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
		return textIntroductionDeliveryKey{}, time.Time{}, &textIntroductionRefusal{cause: errors.New("text Introduction dispatch input unavailable")}
	}
	select {
	case <-registered.channel.Done():
		return textIntroductionDeliveryKey{}, time.Time{}, errors.New("text Introduction registration ended")
	default:
	}
	if err := owner.reserveTextIntroductionOpeningLocked(capsule.DeliveryNonce, now); err != nil {
		return textIntroductionDeliveryKey{}, time.Time{}, &textIntroductionRefusal{cause: err}
	}
	plaintext, _, err := route.OpenClosedIntroduction(capsule, profile.Digest, registered.recipient, now)
	if err != nil {
		return textIntroductionDeliveryKey{}, time.Time{}, &textIntroductionRefusal{cause: err}
	}
	key := textIntroductionDeliveryKey{connection: plaintext.ConnectionNonce, generation: plaintext.AttachmentGeneration}
	clear(plaintext.JoinSecret[:])
	clear(plaintext.HandshakeContext[:])
	if key.connection == [32]byte{} || key.generation == 0 {
		return textIntroductionDeliveryKey{}, time.Time{}, &textIntroductionRefusal{cause: errors.New("text Introduction dispatch identity unavailable")}
	}
	return key, capsule.Expiry, nil
}

func textIntroductionDeliveryMatches(key, want textIntroductionDeliveryKey) bool {
	return key.generation == want.generation && (want.generation == 1 || key.connection == want.connection)
}

func (owner *textContext) takePendingTextIntroductionLocked(want textIntroductionDeliveryKey,
	now time.Time) (textIntroductionPendingDelivery, bool) {
	owner.pruneTextIntroductionPendingLocked(now)
	for key, pending := range owner.introductionPending {
		if !textIntroductionDeliveryMatches(key, want) || len(pending) == 0 {
			continue
		}
		delivery := pending[0]
		if len(pending) == 1 {
			delete(owner.introductionPending, key)
		} else {
			owner.introductionPending[key] = pending[1:]
		}
		return delivery, true
	}
	return textIntroductionPendingDelivery{}, false
}

func (owner *textContext) pruneTextIntroductionPendingLocked(now time.Time) {
	for key, pending := range owner.introductionPending {
		kept := pending[:0]
		for _, item := range pending {
			if now.Before(item.expires) {
				kept = append(kept, item)
			}
		}
		if len(kept) == 0 {
			delete(owner.introductionPending, key)
		} else {
			owner.introductionPending[key] = kept
		}
	}
}

func (owner *textContext) textIntroductionPendingCountLocked() int {
	total := 0
	for _, pending := range owner.introductionPending {
		total += len(pending)
	}
	return total
}
