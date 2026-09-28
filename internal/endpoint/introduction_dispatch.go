//go:build linux

package endpoint

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	introductioncapsule "github.com/dianabuilds/ardents-network/internal/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

const (
	maximumIntroductionWaiters = 16
	introductionExpiryReserve  = 250 * time.Millisecond
)

type introductionDeliveryKey struct {
	connection [32]byte
	generation uint64
}

type introductionRoutedDelivery struct {
	delivery *client.ClosedIntroductionDelivery
	key      introductionDeliveryKey
	expires  time.Time
}

type introductionWaiter struct {
	want     introductionDeliveryKey
	delivery chan introductionRoutedDelivery
}

// receive registers the exact local owner before it
// competes for the one consumer gate. The consumer routes each claimed capsule
// directly to an already registered waiter; unmatched inputs are refused while
// completion is still possible and never occupy registration capacity.
func (dispatch *introductionDispatch) receive(owner *textContext, ctx context.Context, job *textJobIdentity,
	want introductionDeliveryKey, binding *serviceBinding) (delivery introductionRoutedDelivery, outcome error) {
	waiter, gate, err := dispatch.registerWaiter(owner, ctx, job, want, binding)
	if err != nil {
		return introductionRoutedDelivery{}, err
	}
	defer func() {
		outcome = errors.Join(outcome, dispatch.releaseWaiter(owner, waiter))
		if outcome != nil {
			delivery = introductionRoutedDelivery{}
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
			return introductionRoutedDelivery{}, ctx.Err()
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
			claimed, key, expires, err := owner.claimIntroductionDelivery(ctx, job)
			if err != nil {
				gate <- struct{}{}
				return introductionRoutedDelivery{}, err
			}
			routed := introductionRoutedDelivery{delivery: claimed, key: key, expires: expires}
			owner.mu.Lock()
			target := dispatch.selectWaiterLocked(key)
			recovery := (*introductionRecoveryOwner)(nil)
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
			if err := owner.refuseIntroductionDelivery(claimed, expires); err != nil {
				return introductionRoutedDelivery{}, err
			}
			break
		}
	}
}

func (dispatch *introductionDispatch) registerWaiter(owner *textContext, ctx context.Context, job *textJobIdentity,
	want introductionDeliveryKey, binding *serviceBinding) (*introductionWaiter, chan struct{}, error) {
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
	if !owner.liveServiceJobLocked(job, broker.Administration) || ctx.Err() != nil {
		return nil, nil, errors.Join(ctx.Err(), errors.New("text Introduction dispatch owner retired"))
	}
	if dispatch.waiterCapacityReachedLocked() {
		return nil, nil, errors.New("text Introduction dispatch capacity unavailable")
	}
	var recovery *introductionRecoveryOwner
	if want.generation > 1 {
		recovery = binding.dispatchRecoveryLocked(owner, job)
		if recovery == nil {
			return nil, nil, errors.New("text Introduction recovery owner unavailable")
		}
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
func (dispatch *introductionDispatch) releaseWaiter(owner *textContext, waiter *introductionWaiter) error {
	if owner == nil || waiter == nil {
		return nil
	}
	owner.mu.Lock()
	dispatch.removeWaiterLocked(waiter)
	var routed introductionRoutedDelivery
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

func (owner *textContext) retainIntroductionRecovery(binding *serviceBinding) error {
	if owner == nil || !binding.servesOwnerJob(owner) {
		return errors.New("text Introduction recovery owner unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if !owner.liveServiceJobLocked(binding.jobIdentity(), broker.Administration) || binding.hasRecoveryLocked() ||
		owner.introduction.dispatch.recoveryCapacityReachedLocked(owner.streamConnectionLimitLocked()) {
		return errors.New("text Introduction recovery owner capacity unavailable")
	}
	owner.introduction.dispatch.addRecoveryLocked(binding)
	return nil
}

func (binding *serviceBinding) releaseIntroductionRecovery() error {
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
	owner.introduction.dispatch.removeRecoveryLocked(recovery)
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

func (owner *textContext) claimIntroductionDelivery(ctx context.Context,
	job *textJobIdentity) (*client.ClosedIntroductionDelivery, introductionDeliveryKey, time.Time, error) {
	delivery, err := owner.nextIntroductionDelivery(ctx)
	if err != nil {
		return nil, introductionDeliveryKey{}, time.Time{}, err
	}
	key, expires, err := owner.inspectIntroductionDelivery(ctx, job, delivery)
	if err != nil {
		completeErr := owner.refuseIntroductionDelivery(delivery, expires)
		return nil, introductionDeliveryKey{}, time.Time{}, errors.Join(err, completeErr)
	}
	return delivery, key, expires, nil
}

func (owner *textContext) refuseIntroductionDelivery(delivery *client.ClosedIntroductionDelivery, expires time.Time) error {
	return owner.completeIntroductionDelivery(delivery, expires, 1)
}

func (owner *textContext) completeIntroductionDelivery(delivery *client.ClosedIntroductionDelivery,
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

// inspectIntroductionDelivery decrypts only enough capsule state to route
// ownership. It consumes the existing cryptographic-opening rate reservation
// before decryption; acceptance repeats every other authority, publication,
// replay and bound check before acknowledging success.
func (owner *textContext) inspectIntroductionDelivery(ctx context.Context, job *textJobIdentity,
	delivery *client.ClosedIntroductionDelivery) (introductionDeliveryKey, time.Time, error) {
	operation := delivery.Operation()
	defer clear(operation)
	_, capsule, err := introductioncapsule.DecodeSubmission(operation)
	if err != nil {
		return introductionDeliveryKey{}, time.Time{}, &introductionRefusal{cause: err}
	}
	defer clear(capsule.Ciphertext)
	endpoint := owner.endpoint
	endpoint.publisherMu.Lock()
	defer endpoint.publisherMu.Unlock()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.permissionProfileLocked()
	registered := owner.publication.pair.selectLocked(now, capsule.Slot, capsule.Revision)
	if err != nil || ctx.Err() != nil || !owner.liveServiceJobLocked(job, broker.Administration) ||
		registered == nil || owner.publication.pair.withdrawalInProgressLocked() || endpoint.publisherOwner != owner ||
		!endpoint.publicationLive || endpoint.publisherBinding == nil || endpoint.publications == nil ||
		!registered.acceptingNowLocked() || !registered.matchesRequest(capsule.Slot, capsule.Revision) ||
		!now.Add(introductionExpiryReserve).Before(capsule.Expiry) || capsule.Expiry.After(registered.expiry()) {
		return introductionDeliveryKey{}, capsule.Expiry, &introductionRefusal{cause: errors.New("text Introduction dispatch input unavailable")}
	}
	if registered.ended() {
		return introductionDeliveryKey{}, capsule.Expiry, fmt.Errorf("text Introduction registration ended: %s", registered.endReason())
	}
	if err := owner.introduction.admission.reserveOpeningLocked(capsule.DeliveryNonce, now); err != nil {
		return introductionDeliveryKey{}, capsule.Expiry, &introductionRefusal{cause: err}
	}
	plaintext, _, err := registered.openCapsuleLocked(capsule, profile.Digest, now)
	if err != nil {
		return introductionDeliveryKey{}, capsule.Expiry, &introductionRefusal{cause: err}
	}
	key := introductionDeliveryKey{connection: plaintext.ConnectionNonce, generation: plaintext.AttachmentGeneration}
	clear(plaintext.JoinSecret[:])
	clear(plaintext.HandshakeContext[:])
	if key.connection == [32]byte{} || key.generation == 0 {
		return introductionDeliveryKey{}, capsule.Expiry, &introductionRefusal{cause: errors.New("text Introduction dispatch identity unavailable")}
	}
	return key, capsule.Expiry, nil
}

func introductionDeliveryMatches(key, want introductionDeliveryKey) bool {
	return key.generation == want.generation && (want.generation == 1 || key.connection == want.connection)
}
