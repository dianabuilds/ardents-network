package connection

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/reachability"
	"github.com/dianabuilds/ardents-network/internal/domain/route"
	"github.com/dianabuilds/ardents-network/internal/domain/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/domain/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/domain/route/join"
	"github.com/dianabuilds/ardents-network/internal/domain/route/prefix"
)

// openReplacement uses the SAME original Source acquisition for this finite
// proposal's current lookup, recipient, fresh capsule and actual JOIN. The
// caller reserved its ticket before any acquisition or irreversible spend.
func (source *InitiatorSource) openReplacement(proposal *initiatorReplacement) (*InitiatorBinding, *join.Joined, error) {
	if err := proposal.check(); err != nil {
		return nil, nil, err
	}
	handle, err := source.scope.Acquire(proposal.ctx)
	if err != nil {
		return nil, nil, errors.Join(err, source.scope.RetiredWork())
	}
	proposal.sourceHandle = &initiatorRouteAcquisition{handle: handle}
	acquisition, err := join.AcquireScopedSourceJoin(proposal.stream.owner.context(), handle)
	if err != nil {
		return nil, nil, err
	}
	proposal.acquisition = acquisition
	proposal.stopInterruption = context.AfterFunc(proposal.ctx, func() {
		defer close(proposal.interruptionDone)
		proposal.interruptErr = acquisition.Close()
	})
	lookupEnd := proposal.ticket.Deadline().UTC().Truncate(time.Second)
	candidate, err := handle.LookupDescriptor(proposal.ctx, lookupEnd, source.target, source.config.History, source.config.Exclusions)
	if err != nil {
		return nil, nil, fmt.Errorf("connection replacement Descriptor lookup: %w", err)
	}
	descriptor, err := reachability.Verify(candidate.Bytes(), source.target, source.config.Network, source.config.Profile, time.Now())
	if err != nil {
		return nil, nil, err
	}
	stream := proposal.stream
	stream.mu.Lock()
	revision := stream.recipientRevision
	stream.mu.Unlock()
	value := descriptor.Publication.Delegation()
	identity := stream.owner.identity
	if descriptor.PublicationDigest != identity.Publication || value.Instance != identity.Instance || value.Generation != identity.InstanceGeneration || descriptor.Introduction.Revision < revision {
		return nil, nil, proposal.failIntegrity(nil)
	}
	if err := errors.Join(proposal.check(), descriptor.Current(time.Now())); err != nil {
		return nil, nil, err
	}
	intro, err := handle.ResolveSubmission(descriptor.Introduction.Node, source.config.Exclusions)
	if err != nil {
		return nil, nil, err
	}
	excluded := append(append([]route.Member(nil), source.config.Exclusions...), route.Member{NodeID: intro.NodeID, PublicKey: intro.PublicKey, FamilyID: intro.FamilyID})
	// At most the existing alternate is used; failed attempts cannot redraw or
	// expand the original context's retained Rendezvous set.
	rendezvous, until, err := handle.Rendezvous(source.config.Rendezvous, proposal.slot, excluded)
	if err != nil || source.config.WorkSafetyMaximum.After(until) {
		return nil, nil, errors.Join(errIntegrity, err)
	}
	if err := handle.CheckRendezvous(rendezvous, excluded); err != nil {
		return nil, nil, err
	}
	if source.config.PrepareRoute != nil {
		if err := source.config.PrepareRoute(proposal.ctx, handle, intro, rendezvous); err != nil {
			return nil, nil, fmt.Errorf("connection replacement Route preparation: %w", err)
		}
	}
	setupEnd := attachmentSetupDeadline(proposal.ctx, time.Now(), lookupEnd, source.config.WorkSafetyNotAfter, descriptor.Introduction.NotAfter)
	if !time.Now().Before(setupEnd) {
		return nil, nil, errRecoveryExhausted
	}
	request := capsule.Request{Network: identity.Network, Target: identity.Target, PublicationDigest: identity.Publication,
		Revision: descriptor.Introduction.Revision, RendezvousNode: rendezvous.NodeID, RendezvousDutyGeneration: rendezvous.RecordGeneration,
		ProfileDigest: identity.Profile, AttachmentGeneration: proposal.ticket.Generation(), Deadline: setupEnd,
		ConnectionNonce: identity.ConnectionNonce, InitiatorBinding: identity.InitiatorBinding,
		WorkSafetyNotAfter: identity.WorkSafetyNotAfter, WorkSafetyMaximum: identity.WorkSafetyMaximum, NoNewRecoveryAfter: identity.NoNewRecoveryAfter}
	defer clear(request.JoinSecret[:])
	if _, err := rand.Read(request.JoinSecret[:]); err != nil {
		return nil, nil, err
	}
	if _, err := rand.Read(request.HandshakeContext[:]); err != nil {
		return nil, nil, err
	}
	digest, err := request.Digest()
	if err != nil {
		return nil, nil, err
	}
	derived, err := deriveRecipientGeneration(descriptor.Publication, identity.Profile, request, digest, source.config.WorkSafetyMaximum, time.Now(), proposal.ticket.Generation(), identity.Application)
	if err != nil || derived.identity != identity || derived.logical != stream.owner.logical {
		return nil, nil, errors.Join(errIntegrity, err)
	}
	binding := &InitiatorBinding{operation: source.config.Operation, setup: proposal.ctx, identity: derived.identity, logical: derived.logical,
		attachment: derived.attachment, localContext: source.localContext, expires: setupEnd, revision: request.Revision,
		recipientEnd: descriptor.Introduction.NotAfter, proof: descriptor.Publication, source: source,
		joinSecretDigest: sha256.Sum256(request.JoinSecret[:]), joinContext: request.HandshakeContext}
	binding.self = binding
	header := capsule.Header{Slot: descriptor.Introduction.Slot, Revision: request.Revision, Expiry: setupEnd}
	if _, err := rand.Read(header.DeliveryNonce[:]); err != nil {
		return nil, nil, err
	}
	envelope, _, err := capsule.Seal(header, descriptor.Introduction.RecipientKey, request)
	if err != nil {
		return nil, nil, err
	}
	if err := errors.Join(proposal.check(), binding.check(true)); err != nil {
		return nil, nil, err
	}
	setup, cancel := context.WithDeadline(proposal.ctx, setupEnd)
	defer cancel()
	var physical *join.Joined
	err = runInitiatorSetupFlights(setup, acquisition.Close, func(ctx context.Context) error {
		if err := introduction.SubmitScoped(ctx, handle, intro, envelope, source.config.Exclusions); err != nil {
			return fmt.Errorf("connection replacement Introduction submission: %w", err)
		}
		return nil
	}, func() error {
		var err error
		physical, err = acquisition.Join(stream.owner.context(), join.Config{Duty: rendezvous,
			Secret: request.JoinSecret, Context: request.HandshakeContext, Deadline: source.config.WorkSafetyNotAfter, SetupDeadline: setupEnd})
		if err != nil {
			return fmt.Errorf("connection replacement JOIN: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if err := errors.Join(proposal.check(), binding.check(true), descriptor.Current(time.Now()), setup.Err()); err != nil {
		return nil, nil, err
	}
	return binding, physical, nil
}

// The exact physical handle stays within this permitted Route import owner.
type initiatorRouteAcquisition struct{ handle *prefix.SourceHandle }

func (route *initiatorRouteAcquisition) Commit(action func(func() error) error) error {
	if route == nil || route.handle == nil {
		return errIntegrity
	}
	return route.handle.Commit(action)
}

func (route *initiatorRouteAcquisition) close() {
	if route != nil {
		route.handle.Close()
	}
}
