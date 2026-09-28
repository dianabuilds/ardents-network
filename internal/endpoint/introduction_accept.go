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
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

func (owner *dutyContext) acceptIntroductionGeneration(ctx context.Context, job *jobIdentity, operation []byte,
	original *serviceBinding, expectedGeneration uint64, recoveryDeadline time.Time,
	openingReserved bool) (attempt *introductionAttempt, outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("text Introduction caller unavailable")
	}
	_, capsule, err := introductioncapsule.DecodeSubmission(operation)
	if err != nil {
		return nil, introduction.NewRefusal(err)
	}
	defer clear(capsule.Ciphertext)
	endpoint := owner.endpoint
	endpoint.publisherMu.Lock()
	defer endpoint.publisherMu.Unlock()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.publication.pair.DrainingLocked() {
		return nil, errPublicationDraining
	}
	profile, now, err := owner.permissionProfileLocked()
	registered := owner.publication.pair.SelectLocked(now, capsule.Slot, capsule.Revision)
	if err != nil || !owner.liveServiceJobLocked(job, broker.Administration) || registered == nil || owner.publication.pair.WithdrawalInProgressLocked() ||
		endpoint.publisherOwner != owner || !endpoint.publicationLive || endpoint.publisherBinding == nil || endpoint.publications == nil ||
		!registered.AcceptingNowLocked() {
		return nil, errors.New("text Introduction registration authority unavailable")
	}
	if !registered.MatchesRequest(capsule.Slot, capsule.Revision) || !now.Before(capsule.Expiry) || capsule.Expiry.After(registered.Expiry()) {
		return nil, introduction.NewRefusal(errors.New("text Introduction registration input mismatch"))
	}
	if registered.Ended() {
		return nil, fmt.Errorf("text Introduction registration ended: %s", registered.EndReason())
	}
	if !openingReserved {
		if err := owner.introduction.admission.ReserveOpeningLocked(capsule.DeliveryNonce, now); err != nil {
			return nil, introduction.NewRefusal(err)
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
	verified, err := registered.VerifyDescriptorLocked(current.Credential.Target, profile.NetworkID, profile.Digest, now)
	if err != nil || verified.Current.Digest != current.Digest || verified.Current.Credential != current.Credential ||
		verified.Descriptor.Private.Slot != capsule.Slot || verified.Descriptor.Private.Revision != capsule.Revision ||
		verified.Descriptor.Private.RecipientKey != registered.RecipientPublicLocked(now) {
		return nil, errors.New("text Introduction private publication changed")
	}
	plaintext, digest, err := registered.OpenCapsuleLocked(capsule, profile.Digest, now)
	if err != nil {
		return nil, introduction.NewRefusal(err)
	}
	node, generation, until, err := owner.introductionRecipientLocked()
	if err != nil || ctx.Err() != nil || !owner.liveServiceJobLocked(job, broker.Administration) {
		return nil, errors.Join(err, ctx.Err(), errors.New("text Introduction recipient authority unavailable"))
	}
	if node != plaintext.RendezvousNode || generation != plaintext.RendezvousDutyGeneration || plaintext.Deadline.After(until) ||
		plaintext.Network != profile.NetworkID || plaintext.Target != current.Credential.Target || plaintext.PublicationDigest != current.Digest ||
		plaintext.AttachmentGeneration != expectedGeneration || !recoveryDeadline.IsZero() && plaintext.Deadline.After(recoveryDeadline) {
		return nil, introduction.NewRefusal(errors.New("text Introduction recipient facts unavailable"))
	}
	facts := nativeconnection.ProtectedContextInput{Network: plaintext.Network, Target: plaintext.Target, PublicationDigest: plaintext.PublicationDigest,
		InstancePublic: current.Credential.InstancePublic, InstanceGeneration: current.Credential.Generation, ProfileDigest: plaintext.ProfileDigest,
		ConnectionNonce: plaintext.ConnectionNonce, InitiatorBinding: plaintext.InitiatorBinding, WorkSafetyNotAfter: plaintext.WorkSafetyNotAfter,
		WorkSafetyMaximum: plaintext.WorkSafetyMaximum, NoNewRecoveryAfter: plaintext.NoNewRecoveryAfter}
	binding, err := owner.bindServiceLocked(job, current, facts)
	if err != nil {
		return nil, err
	}
	if original != nil {
		if !binding.sameAuthorityAs(original) {
			return nil, introduction.NewRefusal(errors.New("text recovery changed logical Service authority"))
		}
		binding = original
	}
	if registered.Ended() {
		return nil, fmt.Errorf("text Introduction registration ended during opening: %s", registered.EndReason())
	}
	at := endpoint.clock().UTC()
	retained := owner.publication.pair.RetainedLocked(registered, at)
	if ctx.Err() != nil || !owner.liveServiceJobLocked(job, broker.Administration) || !at.Before(capsule.Expiry) || !retained || registered.RecipientPublicLocked(at) == [32]byte{} {
		return nil, errors.Join(ctx.Err(), errors.New("text Introduction authority ended during binding"))
	}
	owner.introduction.admission.RetainAcceptedLocked(capsule.DeliveryNonce, registered.Expiry())
	return &introductionAttempt{binding: binding, plaintext: plaintext, digest: digest}, nil
}
