//go:build linux

package endpoint

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
	introductioncapsule "github.com/dianabuilds/ardents-network/internal/route/capsule"
)

const (
	maximumTextIntroductionWaiters = 16
	textIntroductionExpiryReserve  = 250 * time.Millisecond
)

type textIntroductionDeliveryKey struct {
	connection [32]byte
	generation uint64
}

type textIntroductionRoutedDelivery struct {
	delivery *route.ClosedIntroductionDelivery
	key      textIntroductionDeliveryKey
	expires  time.Time
}

type textIntroductionWaiter struct {
	want     textIntroductionDeliveryKey
	delivery chan textIntroductionRoutedDelivery
}

// receive registers the exact local owner before it
// competes for the one consumer gate. The consumer routes each claimed capsule
// directly to an already registered waiter; unmatched inputs are refused while
// completion is still possible and never occupy registration capacity.
func (dispatch *textIntroductionDispatch) receive(owner *textContext, ctx context.Context, job *textJobIdentity,
	want textIntroductionDeliveryKey, binding *textServiceBinding) (delivery textIntroductionRoutedDelivery, outcome error) {
	waiter, gate, err := dispatch.registerWaiter(owner, ctx, job, want, binding)
	if err != nil {
		return textIntroductionRoutedDelivery{}, err
	}
	defer func() {
		outcome = errors.Join(outcome, dispatch.releaseWaiter(owner, waiter))
		if outcome != nil {
			delivery = textIntroductionRoutedDelivery{}
		}
	}()
	for {
		select {
		case routed := <-waiter.delivery:
			return routed, nil
		default:
		}
		select {
		case routed := <-waiter.delivery:
			return routed, nil
		case <-ctx.Done():
			return textIntroductionRoutedDelivery{}, ctx.Err()
		case <-gate:
		}
		// Another consumer may have assigned this waiter immediately before it
		// released the gate. Do not block the assigned owner behind a fresh take.
		select {
		case routed := <-waiter.delivery:
			gate <- struct{}{}
			return routed, nil
		default:
		}
		for {
			claimed, key, expires, err := owner.claimTextIntroductionDelivery(ctx, job)
			if err != nil {
				gate <- struct{}{}
				return textIntroductionRoutedDelivery{}, err
			}
			routed := textIntroductionRoutedDelivery{delivery: claimed, key: key, expires: expires}
			owner.mu.Lock()
			target := dispatch.selectWaiterLocked(key)
			recovery := (*textIntroductionRecoveryOwner)(nil)
			if target == nil {
				recovery = dispatch.selectRecoveryLocked(key)
			}
			if target != nil && target != waiter {
				target.delivery <- routed
			} else if recovery != nil && !recovery.bufferLocked(owner, routed) {
				recovery = nil
			}
			owner.mu.Unlock()
			if target == waiter {
				gate <- struct{}{}
				return routed, nil
			}
			if target != nil {
				continue
			}
			if recovery != nil {
				continue
			}
			// No local recovery owns this identity. Refuse it now instead of
			// retaining a claimed lane until Complete can no longer answer it.
			gate <- struct{}{}
			if err := owner.refuseTextIntroductionDelivery(claimed, expires); err != nil {
				return textIntroductionRoutedDelivery{}, err
			}
			break
		}
	}
}

func (dispatch *textIntroductionDispatch) registerWaiter(owner *textContext, ctx context.Context, job *textJobIdentity,
	want textIntroductionDeliveryKey, binding *textServiceBinding) (*textIntroductionWaiter, chan struct{}, error) {
	if owner == nil || ctx == nil || want.generation == 0 {
		return nil, nil, errors.New("text Introduction dispatch unavailable")
	}
	var expiryDone <-chan struct{}
	owner.mu.Lock()
	defer func() {
		owner.mu.Unlock()
		if expiryDone != nil {
			<-expiryDone
		}
	}()
	if !owner.liveTextServiceJobLocked(job, broker.Administration) || ctx.Err() != nil {
		return nil, nil, errors.Join(ctx.Err(), errors.New("text Introduction dispatch owner retired"))
	}
	if dispatch.waiterCapacityReachedLocked() {
		return nil, nil, errors.New("text Introduction dispatch capacity unavailable")
	}
	var recovery *textIntroductionRecoveryOwner
	if want.generation > 1 {
		if binding == nil || binding.owner != owner || binding.job != job || binding.recovery == nil {
			return nil, nil, errors.New("text Introduction recovery owner unavailable")
		}
		recovery = binding.recovery
		if !recovery.admitGenerationLocked(binding, want.generation) {
			return nil, nil, errors.New("text Introduction recovery generation unavailable")
		}
	}
	waiter, gate := dispatch.addWaiterLocked(want)
	if recovery != nil {
		expiryDone = recovery.handoffLocked(want, waiter)
	}
	return waiter, gate, nil
}

// releaseWaiter joins ownership of a delivery assigned at the
// same instant its waiter was canceled. A live context refuses that capsule;
// terminal context cleanup instead closes the whole registration owner.
func (dispatch *textIntroductionDispatch) releaseWaiter(owner *textContext, waiter *textIntroductionWaiter) error {
	if owner == nil || waiter == nil {
		return nil
	}
	owner.mu.Lock()
	dispatch.removeWaiterLocked(waiter)
	var routed textIntroductionRoutedDelivery
	select {
	case routed = <-waiter.delivery:
	default:
	}
	lifetime := owner.lease.Context()
	owner.mu.Unlock()
	if routed.delivery == nil || lifetime.Err() != nil {
		return nil
	}
	return routed.delivery.Complete(lifetime, 1)
}

func (owner *textContext) retainTextIntroductionRecovery(binding *textServiceBinding) error {
	if owner == nil || binding == nil || binding.owner != owner || binding.job == nil {
		return errors.New("text Introduction recovery owner unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if !owner.liveTextServiceJobLocked(binding.job, broker.Administration) || binding.recovery != nil ||
		owner.introductionDispatch.recoveryCapacityReachedLocked(owner.streamConnectionLimitLocked()) {
		return errors.New("text Introduction recovery owner capacity unavailable")
	}
	owner.introductionDispatch.addRecoveryLocked(binding)
	return nil
}

func (binding *textServiceBinding) releaseTextIntroductionRecovery() error {
	if binding == nil || binding.owner == nil {
		return nil
	}
	owner := binding.owner
	owner.mu.Lock()
	recovery := binding.recovery
	if recovery == nil || recovery.binding != binding {
		owner.mu.Unlock()
		return nil
	}
	owner.introductionDispatch.removeRecoveryLocked(recovery)
	binding.recovery = nil
	expiryDone, routed := recovery.retireLocked()
	lifetime := owner.lease.Context()
	owner.mu.Unlock()
	if expiryDone != nil {
		<-expiryDone
	}
	if routed.delivery == nil || lifetime.Err() != nil {
		return nil
	}
	return routed.delivery.Complete(lifetime, 1)
}

func (owner *textContext) claimTextIntroductionDelivery(ctx context.Context,
	job *textJobIdentity) (*route.ClosedIntroductionDelivery, textIntroductionDeliveryKey, time.Time, error) {
	delivery, err := owner.nextTextIntroductionDelivery(ctx)
	if err != nil {
		return nil, textIntroductionDeliveryKey{}, time.Time{}, err
	}
	key, expires, err := owner.inspectTextIntroductionDelivery(ctx, job, delivery)
	if err != nil {
		completeErr := owner.refuseTextIntroductionDelivery(delivery, expires)
		return nil, textIntroductionDeliveryKey{}, time.Time{}, errors.Join(err, completeErr)
	}
	return delivery, key, expires, nil
}

func (owner *textContext) refuseTextIntroductionDelivery(delivery *route.ClosedIntroductionDelivery, expires time.Time) error {
	return owner.completeTextIntroductionDelivery(delivery, expires, 1)
}

func (owner *textContext) completeTextIntroductionDelivery(delivery *route.ClosedIntroductionDelivery,
	expires time.Time, status uint8) error {
	if owner == nil || delivery == nil {
		return errors.New("text Introduction completion owner unavailable")
	}
	lifetime := owner.lease.Context()
	if expires.IsZero() {
		return delivery.Complete(lifetime, status)
	}
	bounded, cancel := context.WithDeadline(lifetime, expires)
	defer cancel()
	return delivery.Complete(bounded, status)
}

// inspectTextIntroductionDelivery decrypts only enough capsule state to route
// ownership. It consumes the existing cryptographic-opening rate reservation
// before decryption; acceptance repeats every other authority, publication,
// replay and bound check before acknowledging success.
func (owner *textContext) inspectTextIntroductionDelivery(ctx context.Context, job *textJobIdentity,
	delivery *route.ClosedIntroductionDelivery) (textIntroductionDeliveryKey, time.Time, error) {
	operation := delivery.Operation()
	defer clear(operation)
	_, capsule, err := introductioncapsule.DecodeSubmission(operation)
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
	registered := owner.textPublicationPairLifecycle.selectLocked(now, capsule.Slot, capsule.Revision)
	if err != nil || ctx.Err() != nil || !owner.liveTextServiceJobLocked(job, broker.Administration) ||
		registered == nil || owner.textPublicationPairLifecycle.withdrawalInProgressLocked() || endpoint.textPublisherOwner != owner ||
		!endpoint.textPublicationLive || endpoint.publisherBinding == nil || endpoint.publications == nil ||
		!registered.published || registered.recipient == nil ||
		capsule.Slot != registered.request.Slot || capsule.Revision != registered.request.Revision ||
		!now.Add(textIntroductionExpiryReserve).Before(capsule.Expiry) || capsule.Expiry.After(registered.request.Expiry) {
		return textIntroductionDeliveryKey{}, capsule.Expiry, &textIntroductionRefusal{cause: errors.New("text Introduction dispatch input unavailable")}
	}
	select {
	case <-registered.channel.Done():
		return textIntroductionDeliveryKey{}, capsule.Expiry, fmt.Errorf("text Introduction registration ended: %s", registered.channel.EndReason())
	default:
	}
	if err := owner.introductionAdmission.reserveOpeningLocked(capsule.DeliveryNonce, now); err != nil {
		return textIntroductionDeliveryKey{}, capsule.Expiry, &textIntroductionRefusal{cause: err}
	}
	plaintext, _, err := introductioncapsule.Open(capsule, profile.Digest, registered.recipient, now)
	if err != nil {
		return textIntroductionDeliveryKey{}, capsule.Expiry, &textIntroductionRefusal{cause: err}
	}
	key := textIntroductionDeliveryKey{connection: plaintext.ConnectionNonce, generation: plaintext.AttachmentGeneration}
	clear(plaintext.JoinSecret[:])
	clear(plaintext.HandshakeContext[:])
	if key.connection == [32]byte{} || key.generation == 0 {
		return textIntroductionDeliveryKey{}, capsule.Expiry, &textIntroductionRefusal{cause: errors.New("text Introduction dispatch identity unavailable")}
	}
	return key, capsule.Expiry, nil
}

func textIntroductionDeliveryMatches(key, want textIntroductionDeliveryKey) bool {
	return key.generation == want.generation && (want.generation == 1 || key.connection == want.connection)
}
