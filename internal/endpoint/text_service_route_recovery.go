//go:build linux

package endpoint

import (
	"context"
	"crypto/rand"
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

// textServiceRouteRecoveryOpener keeps Route replacement behind the native
// Connection's immutable recovery request. The client creates one fresh
// capsule/JOIN; the Publisher accepts only that generation for the retained
// logical binding. Neither side can open another Application operation.
func (owner *textContext) textServiceRouteRecoveryOpener(job *textJobIdentity,
	binding *textServiceBinding) textServiceAttachmentOpener {
	if owner == nil || binding == nil || binding.owner != owner || binding.job != job {
		return nil
	}
	return func(ctx context.Context, request nativeconnection.Recovery) (net.Conn, [32]byte, error) {
		if ctx == nil {
			return nil, [32]byte{}, errors.New("text recovery context unavailable")
		}
		if err := ctx.Err(); err != nil {
			return nil, [32]byte{}, err
		}
		if err := binding.validateTextServiceRecovery(request); err != nil {
			return nil, [32]byte{}, err
		}
		var prepared *textIntroductionAttempt
		var err error
		switch owner.surface {
		case broker.Connection:
			prepared, err = owner.prepareTextRecovery(ctx, job, binding, request)
		case broker.Administration:
			prepared, err = owner.receiveTextRecovery(ctx, job, binding, request)
		default:
			err = errors.New("text recovery surface unavailable")
		}
		if err != nil {
			return nil, [32]byte{}, err
		}
		defer clear(prepared.operation)
		raw, err := owner.openTextJoinedTransport(ctx, job, prepared)
		if err != nil {
			return nil, [32]byte{}, err
		}
		return raw, prepared.digest, nil
	}
}

// prepareTextRecovery resolves the current recipient for the original Target
// and accepts it only under the Connection's immutable Publication authority.
// It creates fresh per-Attachment secrets without resetting any work deadline.
func (owner *textContext) prepareTextRecovery(ctx context.Context, job *textJobIdentity, binding *textServiceBinding,
	request nativeconnection.Recovery) (*textIntroductionAttempt, error) {
	if owner == nil || ctx == nil || binding == nil || binding.owner != owner || binding.job != job ||
		owner.surface != broker.Connection {
		return nil, errors.New("text recovery preparation unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := binding.validateTextServiceRecovery(request); err != nil {
		return nil, err
	}
	verified, err := owner.lookupTextDescriptor(ctx, binding.facts.Target)
	if err != nil {
		return nil, err
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.textPermissionProfileLocked()
	prefix := owner.prefix
	recipient := verified.Descriptor.Private
	floor := owner.descriptorFloors[binding.facts.Target]
	if attemptErr := ctx.Err(); attemptErr != nil {
		return nil, attemptErr
	}
	if err != nil || prefix == nil || !owner.liveTextServiceJobLocked(job, broker.Connection) ||
		!binding.matchesPublication(verified.Current) || verified.Descriptor.ProfileDigest != binding.facts.ProfileDigest ||
		profile.Digest != binding.facts.ProfileDigest || floor.publicationConflict || floor.revisionConflict ||
		floor.publication != binding.facts.PublicationDigest || floor.revision != recipient.Revision ||
		recipient.Revision < binding.introduction.Revision ||
		recipient.Revision == 0 || recipient.Slot == [32]byte{} || recipient.RecipientKey == [32]byte{} ||
		now.Before(recipient.NotBefore) || !now.Before(recipient.NotAfter) {
		return nil, errors.Join(err, errors.New("text recovery recipient or authority unavailable"))
	}
	binding.introduction = recipient
	node, generation, until, err := prefix.DataJoinRecipient()
	if err != nil {
		return nil, err
	}
	deadline := now.Add(10 * time.Second).UTC().Truncate(time.Second)
	for _, bound := range []time.Time{request.Deadline, until, recipient.NotAfter, time.Unix(binding.facts.WorkSafetyNotAfter, 0)} {
		if bound.Before(deadline) {
			deadline = bound.UTC().Truncate(time.Second)
		}
	}
	if !now.Before(deadline) {
		return nil, errors.New("text recovery deadline unavailable")
	}
	facts := binding.facts
	plaintext := route.ClosedIntroductionPlaintext{Network: facts.Network, Target: facts.Target,
		PublicationDigest: facts.PublicationDigest, Revision: recipient.Revision, RendezvousNode: node,
		RendezvousDutyGeneration: generation, ProfileDigest: facts.ProfileDigest, ConnectionNonce: facts.ConnectionNonce,
		AttachmentGeneration: request.Generation, Deadline: deadline, InitiatorBinding: facts.InitiatorBinding,
		WorkSafetyNotAfter: facts.WorkSafetyNotAfter, WorkSafetyMaximum: facts.WorkSafetyMaximum,
		NoNewRecoveryAfter: facts.NoNewRecoveryAfter}
	capsule := route.ClosedIntroductionCapsule{Slot: recipient.Slot, Revision: recipient.Revision, Expiry: deadline}
	var requestNonce [32]byte
	for _, value := range []*[32]byte{&plaintext.JoinSecret, &plaintext.HandshakeContext, &capsule.DeliveryNonce, &requestNonce} {
		if _, err := rand.Read(value[:]); err != nil {
			return nil, err
		}
	}
	capsule, digest, err := route.SealClosedIntroduction(capsule, recipient.RecipientKey, plaintext)
	if err != nil {
		return nil, err
	}
	operation, err := route.EncodeClosedIntroductionSubmission(requestNonce, capsule)
	if err != nil {
		return nil, err
	}
	if attemptErr := ctx.Err(); attemptErr != nil {
		clear(operation)
		return nil, attemptErr
	}
	if owner.prefix != prefix || !owner.liveTextServiceJobLocked(job, broker.Connection) ||
		!owner.endpoint.clock().UTC().Before(deadline) {
		clear(operation)
		return nil, errors.New("text recovery authority ended during sealing")
	}
	return &textIntroductionAttempt{binding: binding, plaintext: plaintext, operation: operation, digest: digest}, nil
}
