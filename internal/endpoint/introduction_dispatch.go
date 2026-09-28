//go:build linux

package endpoint

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/endpoint/introduction"
	introductioncapsule "github.com/dianabuilds/ardents-network/internal/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// receiveIntroductionDelivery registers the exact local owner before it
// competes for the one consumer gate. The consumer routes each claimed capsule
// directly to an already registered waiter; unmatched inputs are refused while
// completion is still possible and never occupy registration capacity. The
// slot mechanism itself lives in the introduction subpackage; this loop is
// root orchestration because it needs live job, lease, and publication
// authority.
func (owner *dutyContext) receiveIntroductionDelivery(ctx context.Context, job *jobIdentity,
	want introduction.DeliveryKey, binding *serviceBinding) (delivery introduction.RoutedDelivery, outcome error) {
	dispatch := &owner.introduction.dispatch
	waiter, gate, err := owner.registerIntroductionWaiter(ctx, job, want, binding)
	if err != nil {
		return introduction.RoutedDelivery{}, err
	}
	defer func() {
		outcome = errors.Join(outcome, owner.releaseIntroductionWaiter(waiter))
		if outcome != nil {
			delivery = introduction.RoutedDelivery{}
		}
	}()
	for {
		select {
		case routed := <-waiter.Delivery:
			return routed, nil
		default:
		}
		select {
		case routed := <-waiter.Delivery:
			return routed, nil
		case <-ctx.Done():
			return introduction.RoutedDelivery{}, ctx.Err()
		case <-gate:
		}
		// Another consumer may have assigned this waiter immediately before it
		// released the gate. Do not block the assigned owner behind a fresh take.
		select {
		case routed := <-waiter.Delivery:
			gate <- struct{}{}
			return routed, nil
		default:
		}
		for {
			claimed, key, expires, err := owner.claimIntroductionDelivery(ctx, job)
			if err != nil {
				gate <- struct{}{}
				return introduction.RoutedDelivery{}, err
			}
			routed := introduction.RoutedDelivery{Delivery: claimed, Key: key, Expires: expires}
			owner.mu.Lock()
			target := dispatch.SelectWaiterLocked(key)
			recovery := (*introduction.RecoveryOwner)(nil)
			if target == nil {
				recovery = dispatch.SelectRecoveryLocked(key)
			}
			if target != nil && target != waiter {
				target.Delivery <- routed
			} else if recovery != nil && !recovery.BufferLocked(dutyIntroductionHost{owner}, routed) {
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
				return introduction.RoutedDelivery{}, err
			}
			break
		}
	}
}

func (owner *dutyContext) registerIntroductionWaiter(ctx context.Context, job *jobIdentity,
	want introduction.DeliveryKey, binding *serviceBinding) (*introduction.Waiter, chan struct{}, error) {
	if owner == nil || ctx == nil || want.Generation == 0 {
		return nil, nil, errors.New("text Introduction dispatch unavailable")
	}
	dispatch := &owner.introduction.dispatch
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
	if dispatch.WaiterCapacityReachedLocked() {
		return nil, nil, errors.New("text Introduction dispatch capacity unavailable")
	}
	var recovery *introduction.RecoveryOwner
	if want.Generation > 1 {
		recovery = binding.dispatchRecoveryLocked(owner, job)
		if recovery == nil {
			return nil, nil, errors.New("text Introduction recovery owner unavailable")
		}
		if !recovery.AdmitGenerationLocked(binding, want.Generation) {
			return nil, nil, errors.New("text Introduction recovery generation unavailable")
		}
	}
	waiter, gate := dispatch.AddWaiterLocked(want)
	if recovery != nil {
		expiryDone = recovery.HandoffLocked(want, waiter)
	}
	return waiter, gate, nil
}

// releaseIntroductionWaiter joins ownership of a delivery assigned at the
// same instant its waiter was canceled. A live context refuses that capsule;
// terminal context cleanup instead closes the whole registration owner.
func (owner *dutyContext) releaseIntroductionWaiter(waiter *introduction.Waiter) error {
	if owner == nil || waiter == nil {
		return nil
	}
	owner.mu.Lock()
	owner.introduction.dispatch.RemoveWaiterLocked(waiter)
	var routed introduction.RoutedDelivery
	select {
	case routed = <-waiter.Delivery:
	default:
	}
	lifetime := owner.lease.Context()
	owner.mu.Unlock()
	if routed.Delivery == nil || lifetime.Err() != nil {
		return nil
	}
	return routed.Delivery.Complete(lifetime, 1)
}

func (owner *dutyContext) retainIntroductionRecovery(binding *serviceBinding) error {
	if owner == nil || !binding.servesOwnerJob(owner) {
		return errors.New("text Introduction recovery owner unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if !owner.liveServiceJobLocked(binding.jobIdentity(), broker.Administration) || binding.hasRecoveryLocked() ||
		owner.introduction.dispatch.RecoveryCapacityReachedLocked(owner.streamConnectionLimitLocked()) {
		return errors.New("text Introduction recovery owner capacity unavailable")
	}
	owner.introduction.dispatch.AddRecoveryLocked(binding.claimRecoveryLocked())
	return nil
}

func (binding *serviceBinding) releaseIntroductionRecovery() error {
	if binding == nil || binding.owner == nil {
		return nil
	}
	owner := binding.owner
	owner.mu.Lock()
	recovery := binding.recovery
	if recovery == nil || !recovery.OwnedBy(binding) {
		owner.mu.Unlock()
		return nil
	}
	owner.introduction.dispatch.RemoveRecoveryLocked(recovery)
	binding.recovery = nil
	expiryDone, routed := recovery.RetireLocked()
	lifetime := owner.lease.Context()
	owner.mu.Unlock()
	if expiryDone != nil {
		<-expiryDone
	}
	if routed.Delivery == nil || lifetime.Err() != nil {
		return nil
	}
	return routed.Delivery.Complete(lifetime, 1)
}

func (owner *dutyContext) claimIntroductionDelivery(ctx context.Context,
	job *jobIdentity) (*client.ClosedIntroductionDelivery, introduction.DeliveryKey, time.Time, error) {
	delivery, err := owner.nextIntroductionDelivery(ctx)
	if err != nil {
		return nil, introduction.DeliveryKey{}, time.Time{}, err
	}
	key, expires, err := owner.inspectIntroductionDelivery(ctx, job, delivery)
	if err != nil {
		completeErr := owner.refuseIntroductionDelivery(delivery, expires)
		return nil, introduction.DeliveryKey{}, time.Time{}, errors.Join(err, completeErr)
	}
	return delivery, key, expires, nil
}

func (owner *dutyContext) refuseIntroductionDelivery(delivery *client.ClosedIntroductionDelivery, expires time.Time) error {
	return owner.completeIntroductionDelivery(delivery, expires, 1)
}

func (owner *dutyContext) completeIntroductionDelivery(delivery *client.ClosedIntroductionDelivery,
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
func (owner *dutyContext) inspectIntroductionDelivery(ctx context.Context, job *jobIdentity,
	delivery *client.ClosedIntroductionDelivery) (introduction.DeliveryKey, time.Time, error) {
	operation := delivery.Operation()
	defer clear(operation)
	_, capsule, err := introductioncapsule.DecodeSubmission(operation)
	if err != nil {
		return introduction.DeliveryKey{}, time.Time{}, introduction.NewRefusal(err)
	}
	defer clear(capsule.Ciphertext)
	endpoint := owner.endpoint
	endpoint.publisherMu.Lock()
	defer endpoint.publisherMu.Unlock()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.permissionProfileLocked()
	registered := owner.publication.pair.SelectLocked(now, capsule.Slot, capsule.Revision)
	if err != nil || ctx.Err() != nil || !owner.liveServiceJobLocked(job, broker.Administration) ||
		registered == nil || owner.publication.pair.WithdrawalInProgressLocked() || endpoint.publisherOwner != owner ||
		!endpoint.publicationLive || endpoint.publisherBinding == nil || endpoint.publications == nil ||
		!registered.AcceptingNowLocked() || !registered.MatchesRequest(capsule.Slot, capsule.Revision) ||
		!now.Add(introduction.ExpiryReserve).Before(capsule.Expiry) || capsule.Expiry.After(registered.Expiry()) {
		return introduction.DeliveryKey{}, capsule.Expiry, introduction.NewRefusal(errors.New("text Introduction dispatch input unavailable"))
	}
	if registered.Ended() {
		return introduction.DeliveryKey{}, capsule.Expiry, fmt.Errorf("text Introduction registration ended: %s", registered.EndReason())
	}
	if err := owner.introduction.admission.ReserveOpeningLocked(capsule.DeliveryNonce, now); err != nil {
		return introduction.DeliveryKey{}, capsule.Expiry, introduction.NewRefusal(err)
	}
	plaintext, _, err := registered.OpenCapsuleLocked(capsule, profile.Digest, now)
	if err != nil {
		return introduction.DeliveryKey{}, capsule.Expiry, introduction.NewRefusal(err)
	}
	key := introduction.DeliveryKey{Connection: plaintext.ConnectionNonce, Generation: plaintext.AttachmentGeneration}
	clear(plaintext.JoinSecret[:])
	clear(plaintext.HandshakeContext[:])
	if key.Connection == [32]byte{} || key.Generation == 0 {
		return introduction.DeliveryKey{}, capsule.Expiry, introduction.NewRefusal(errors.New("text Introduction dispatch identity unavailable"))
	}
	return key, capsule.Expiry, nil
}
