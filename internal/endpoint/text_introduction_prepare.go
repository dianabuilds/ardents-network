//go:build linux

package endpoint

import (
	"context"
	"crypto/rand"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// textIntroductionAttempt keeps recipient-only join facts and the exact shared
// Service binding local to one job. It is preparation, not a joined Attachment.
type textIntroductionAttempt struct {
	submitted bool
	joined    bool
	binding   *textServiceBinding
	plaintext route.ClosedIntroductionPlaintext
	operation []byte
	digest    [32]byte
}

// prepareTextIntroduction consumes real resolution and its retained conflict
// floors. Neither the worker nor a Descriptor selects Rendezvous or supplies
// a join secret, local-context identifier, HPKE input or shared authority tuple.
func (owner *textContext) prepareTextIntroduction(ctx context.Context, job *textJobIdentity, destination targetlink.Link, bounds [3]int64) (prepared *textIntroductionAttempt, outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("text Introduction caller unavailable")
	}
	if _, err := targetlink.Encode(destination); err != nil || destination.Network != owner.endpoint.network {
		return nil, errors.New("text Introduction destination unavailable")
	}
	owner.mu.Lock()
	if err := owner.retireTextPrefixLocked(); err != nil {
		owner.mu.Unlock()
		return nil, err
	}
	live := owner.liveTextServiceJobLocked(job, broker.Connection)
	needPrefix := owner.prefix == nil
	owner.mu.Unlock()
	if !live {
		return nil, errors.New("text Introduction reader job unavailable")
	}
	// Network work belongs to this invocation even while its independently
	// authorized Endpoint context remains live after worker retirement.
	caller := ctx
	attempt, cancel := context.WithCancel(job.context)
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); cancel() })
	defer func() {
		cancel()
		if !stop() {
			<-interrupted
		}
		// The caller may cancel before its scheduled callback executes.
		// Reconcile that cancellation after joining, before handing over bytes.
		if prepared != nil && (caller.Err() != nil || job.context.Err() != nil || !owner.endpoint.clock().UTC().Before(prepared.plaintext.Deadline)) {
			clear(prepared.operation)
			prepared = nil
			outcome = errors.Join(outcome, caller.Err(), job.context.Err(), errors.New("text Introduction preparation ended before handover"))
		}
	}()
	if ctx.Err() != nil {
		cancel()
	}
	ctx = attempt
	if needPrefix {
		if _, err := owner.openTextPrefix(ctx); err != nil {
			return nil, err
		}
	}
	verified, err := owner.lookupTextDescriptor(ctx, destination.Target)
	if err != nil {
		return nil, err
	}
	binding, err := owner.newTextServiceBinding(job, destination, verified.Current, bounds)
	if err != nil {
		return nil, err
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.textPermissionProfileLocked()
	floor := owner.descriptorFloors[destination.Target]
	if err != nil || !owner.liveTextServiceJobLocked(job, broker.Connection) || ctx.Err() != nil || owner.prefix == nil ||
		profile.Digest != verified.Descriptor.ProfileDigest || floor.publicationConflict || floor.revisionConflict ||
		floor.publication != verified.Current.Digest || floor.revision != verified.Descriptor.Private.Revision {
		return nil, errors.New("text Introduction resolution or local authority changed")
	}
	node, generation, until, err := owner.prefix.DataJoinRecipient()
	if err != nil {
		return nil, err
	}
	deadline := now.Add(10 * time.Second).UTC().Truncate(time.Second)
	for _, bound := range []time.Time{until, verified.Descriptor.Private.NotAfter, time.Unix(bounds[0], 0)} {
		if bound.Before(deadline) {
			deadline = bound.UTC().Truncate(time.Second)
		}
	}
	if !now.Before(deadline) {
		return nil, errors.New("text Introduction deadline unavailable")
	}
	facts := binding.facts
	plaintext := route.ClosedIntroductionPlaintext{Network: facts.Network, Target: facts.Target, PublicationDigest: facts.PublicationDigest,
		Revision: verified.Descriptor.Private.Revision, RendezvousNode: node, RendezvousDutyGeneration: generation, ProfileDigest: facts.ProfileDigest,
		ConnectionNonce: facts.ConnectionNonce, AttachmentGeneration: 1, Deadline: deadline, InitiatorBinding: facts.InitiatorBinding,
		WorkSafetyNotAfter: facts.WorkSafetyNotAfter, WorkSafetyMaximum: facts.WorkSafetyMaximum, NoNewRecoveryAfter: facts.NoNewRecoveryAfter}
	capsule := route.ClosedIntroductionCapsule{Slot: verified.Descriptor.Private.Slot, Revision: plaintext.Revision, Expiry: deadline}
	var requestNonce [32]byte
	for _, value := range []*[32]byte{&plaintext.JoinSecret, &plaintext.HandshakeContext, &capsule.DeliveryNonce, &requestNonce} {
		if _, err := rand.Read(value[:]); err != nil {
			return nil, err
		}
	}
	capsule, digest, err := route.SealClosedIntroduction(capsule, verified.Descriptor.Private.RecipientKey, plaintext)
	if err != nil {
		return nil, err
	}
	operation, err := route.EncodeClosedIntroductionSubmission(requestNonce, capsule)
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil || !owner.liveTextServiceJobLocked(job, broker.Connection) || !owner.endpoint.clock().UTC().Before(deadline) {
		clear(operation)
		return nil, errors.New("text Introduction job ended during sealing")
	}
	return &textIntroductionAttempt{binding: binding, plaintext: plaintext, operation: operation, digest: digest}, nil
}
