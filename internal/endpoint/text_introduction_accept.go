//go:build linux

package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// At most four candidates per second over the 600-second registration plus
// the required 60-second replay retention. Entries never move to another
// context; failed/forged opening cannot clear accepted replay history.
const maximumTextIntroductionReplays = 4 * (600 + 60)

// acceptTextIntroduction is the Publisher's pre-dial boundary. It consumes its
// own registration, non-exporting Instance, current Publication and State; the
// returned binding is usable only by this current qualified Publisher job.
func (owner *textContext) acceptTextIntroduction(ctx context.Context, job *textJobIdentity, operation []byte) (attempt *textIntroductionAttempt, outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("text Introduction caller unavailable")
	}
	_, capsule, err := route.DecodeClosedIntroductionSubmission(operation)
	if err != nil {
		return nil, &textIntroductionRefusal{cause: err}
	}
	defer clear(capsule.Ciphertext)
	endpoint := owner.endpoint
	endpoint.publisherMu.Lock()
	defer endpoint.publisherMu.Unlock()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.publicationDraining {
		return nil, errTextPublicationDraining
	}
	profile, now, err := owner.textPermissionProfileLocked()
	registered := owner.registration
	if prior := owner.previousRegistration; prior != nil && now.Before(owner.previousUntil) && prior.request.Slot == capsule.Slot && prior.request.Revision == capsule.Revision {
		registered = prior
	}
	if err != nil || !owner.liveTextServiceJobLocked(job, broker.Administration) || registered == nil || owner.withdrawal != nil ||
		endpoint.textPublisherOwner != owner || !endpoint.textPublicationLive || endpoint.publisherBinding == nil || endpoint.publications == nil ||
		!registered.published || registered.recipient == nil || owner.prefix == nil {
		return nil, errors.New("text Introduction registration authority unavailable")
	}
	if capsule.Slot != registered.request.Slot || capsule.Revision != registered.request.Revision || !now.Before(capsule.Expiry) || capsule.Expiry.After(registered.request.Expiry) {
		return nil, &textIntroductionRefusal{cause: errors.New("text Introduction registration input mismatch")}
	}
	select {
	case <-registered.channel.Done():
		return nil, errors.New("text Introduction registration ended")
	default:
	}
	if err := owner.reserveTextIntroductionOpeningLocked(capsule.DeliveryNonce, now); err != nil {
		return nil, &textIntroductionRefusal{cause: err}
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
	plaintext, digest, err := route.OpenClosedIntroduction(capsule, profile.Digest, registered.recipient, now)
	if err != nil {
		return nil, &textIntroductionRefusal{cause: err}
	}
	node, generation, until, err := owner.prefix.DataJoinRecipient()
	if err != nil || ctx.Err() != nil || !owner.liveTextServiceJobLocked(job, broker.Administration) {
		return nil, errors.Join(err, ctx.Err(), errors.New("text Introduction recipient authority unavailable"))
	}
	if node != plaintext.RendezvousNode || generation != plaintext.RendezvousDutyGeneration || plaintext.Deadline.After(until) ||
		plaintext.Network != profile.NetworkID || plaintext.Target != current.Credential.Target || plaintext.PublicationDigest != current.Digest || plaintext.AttachmentGeneration != 1 {
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
	select {
	case <-registered.channel.Done():
		return nil, errors.New("text Introduction registration ended during opening")
	default:
	}
	at := endpoint.clock().UTC()
	retained := registered == owner.registration || registered == owner.previousRegistration && at.Before(owner.previousUntil)
	if ctx.Err() != nil || !owner.liveTextServiceJobLocked(job, broker.Administration) || !at.Before(capsule.Expiry) || !retained || registered.recipient.Public(at) == [32]byte{} {
		return nil, errors.Join(ctx.Err(), errors.New("text Introduction authority ended during binding"))
	}
	if owner.introductionReplays == nil {
		owner.introductionReplays = make(map[[32]byte]time.Time)
	}
	owner.introductionReplays[capsule.DeliveryNonce] = registered.request.Expiry.Add(60 * time.Second)
	return &textIntroductionAttempt{binding: binding, plaintext: plaintext, digest: digest}, nil
}

func (owner *textContext) reserveTextIntroductionOpeningLocked(nonce [32]byte, now time.Time) error {
	for retained, expiry := range owner.introductionReplays {
		if !now.Before(expiry) {
			delete(owner.introductionReplays, retained)
		}
	}
	if _, replay := owner.introductionReplays[nonce]; replay || len(owner.introductionReplays) >= maximumTextIntroductionReplays {
		return errors.New("text Introduction replay or capacity refusal")
	}
	if now.Before(owner.introductionOpenings[3]) || now.Before(owner.introductionOpenings[0].Add(time.Second)) {
		return errors.New("text Introduction opening rate unavailable")
	}
	copy(owner.introductionOpenings[:3], owner.introductionOpenings[1:])
	owner.introductionOpenings[3] = now
	return nil
}
