//go:build linux

package endpoint

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	introductioncapsule "github.com/dianabuilds/ardents-network/internal/route/capsule"
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func (owner *textContext) acceptDispatchedTextIntroduction(ctx context.Context, job *textJobIdentity,
	operation []byte) (*textIntroductionAttempt, error) {
	return owner.acceptTextIntroductionGeneration(ctx, job, operation, nil, 1, time.Time{}, true)
}

func (owner *textContext) acceptDispatchedTextRecovery(ctx context.Context, job *textJobIdentity, operation []byte,
	original *textServiceBinding, request nativeconnection.Recovery) (*textIntroductionAttempt, error) {
	if original == nil || original.owner != owner || original.job != job {
		return nil, errors.New("text recovery binding unavailable")
	}
	if err := original.validateTextServiceRecovery(request); err != nil {
		return nil, err
	}
	return owner.acceptTextIntroductionGeneration(ctx, job, operation, original, request.Generation, request.Deadline, true)
}

func (owner *textContext) acceptTextIntroductionGeneration(ctx context.Context, job *textJobIdentity, operation []byte,
	original *textServiceBinding, expectedGeneration uint64, recoveryDeadline time.Time,
	openingReserved bool) (attempt *textIntroductionAttempt, outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("text Introduction caller unavailable")
	}
	_, capsule, err := introductioncapsule.DecodeSubmission(operation)
	if err != nil {
		return nil, &textIntroductionRefusal{cause: err}
	}
	defer clear(capsule.Ciphertext)
	endpoint := owner.endpoint
	endpoint.publisherMu.Lock()
	defer endpoint.publisherMu.Unlock()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.textPublicationPairLifecycle.drainingLocked() {
		return nil, errTextPublicationDraining
	}
	profile, now, err := owner.textPermissionProfileLocked()
	registered := owner.textPublicationPairLifecycle.selectLocked(now, capsule.Slot, capsule.Revision)
	if err != nil || !owner.liveTextServiceJobLocked(job, broker.Administration) || registered == nil || owner.withdrawal != nil ||
		endpoint.textPublisherOwner != owner || !endpoint.textPublicationLive || endpoint.publisherBinding == nil || endpoint.publications == nil ||
		!registered.published || registered.recipient == nil {
		return nil, errors.New("text Introduction registration authority unavailable")
	}
	if capsule.Slot != registered.request.Slot || capsule.Revision != registered.request.Revision || !now.Before(capsule.Expiry) || capsule.Expiry.After(registered.request.Expiry) {
		return nil, &textIntroductionRefusal{cause: errors.New("text Introduction registration input mismatch")}
	}
	select {
	case <-registered.channel.Done():
		return nil, fmt.Errorf("text Introduction registration ended: %s", registered.channel.EndReason())
	default:
	}
	if !openingReserved {
		if err := owner.introductionAdmission.reserveOpeningLocked(capsule.DeliveryNonce, now); err != nil {
			return nil, &textIntroductionRefusal{cause: err}
		}
	}
	lease, err := endpoint.publications.AcquireAt(ctx, now)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := lease.Close(); err != nil {
			attempt = nil
			outcome = errors.Join(outcome, err)
		}
	}()
	current := lease.Current()
	verified, err := reachability.VerifyPrivate(registered.descriptor, current.Credential.Target, profile.NetworkID, profile.Digest, now)
	if err != nil || verified.Current.Digest != current.Digest || verified.Current.Credential != current.Credential ||
		verified.Descriptor.Private.Slot != capsule.Slot || verified.Descriptor.Private.Revision != capsule.Revision ||
		verified.Descriptor.Private.RecipientKey != registered.recipient.Public(now) {
		return nil, errors.New("text Introduction private publication changed")
	}
	plaintext, digest, err := introductioncapsule.Open(capsule, profile.Digest, registered.recipient, now)
	if err != nil {
		return nil, &textIntroductionRefusal{cause: err}
	}
	node, generation, until, err := owner.textIntroductionRecipientLocked()
	if err != nil || ctx.Err() != nil || !owner.liveTextServiceJobLocked(job, broker.Administration) {
		return nil, errors.Join(err, ctx.Err(), errors.New("text Introduction recipient authority unavailable"))
	}
	if node != plaintext.RendezvousNode || generation != plaintext.RendezvousDutyGeneration || plaintext.Deadline.After(until) ||
		plaintext.Network != profile.NetworkID || plaintext.Target != current.Credential.Target || plaintext.PublicationDigest != current.Digest ||
		plaintext.AttachmentGeneration != expectedGeneration || !recoveryDeadline.IsZero() && plaintext.Deadline.After(recoveryDeadline) {
		return nil, &textIntroductionRefusal{cause: errors.New("text Introduction recipient facts unavailable")}
	}
	facts := nativeconnection.ProtectedContextInput{Network: plaintext.Network, Target: plaintext.Target, PublicationDigest: plaintext.PublicationDigest,
		InstancePublic: current.Credential.InstancePublic, InstanceGeneration: current.Credential.Generation, ProfileDigest: plaintext.ProfileDigest,
		ConnectionNonce: plaintext.ConnectionNonce, InitiatorBinding: plaintext.InitiatorBinding, WorkSafetyNotAfter: plaintext.WorkSafetyNotAfter,
		WorkSafetyMaximum: plaintext.WorkSafetyMaximum, NoNewRecoveryAfter: plaintext.NoNewRecoveryAfter}
	binding, err := owner.bindTextServiceLocked(job, current, facts)
	if err != nil {
		return nil, err
	}
	if original != nil {
		if binding.logical != original.logical || binding.facts != original.facts || binding.credential != original.credential ||
			binding.candidateView != original.candidateView {
			return nil, &textIntroductionRefusal{cause: errors.New("text recovery changed logical Service authority")}
		}
		binding = original
	}
	select {
	case <-registered.channel.Done():
		return nil, fmt.Errorf("text Introduction registration ended during opening: %s", registered.channel.EndReason())
	default:
	}
	at := endpoint.clock().UTC()
	retained := owner.textPublicationPairLifecycle.retainedLocked(registered, at)
	if ctx.Err() != nil || !owner.liveTextServiceJobLocked(job, broker.Administration) || !at.Before(capsule.Expiry) || !retained || registered.recipient.Public(at) == [32]byte{} {
		return nil, errors.Join(ctx.Err(), errors.New("text Introduction authority ended during binding"))
	}
	owner.introductionAdmission.retainAcceptedLocked(capsule.DeliveryNonce, registered.request.Expiry)
	return &textIntroductionAttempt{binding: binding, plaintext: plaintext, digest: digest}, nil
}
