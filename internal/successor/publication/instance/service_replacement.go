package instance

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
	"github.com/dianabuilds/ardents-network/internal/successor/route/join"
)

// BeginReplacement retains the SAME original logical lease and Instance key.
// It admits only a private opening and genuine fresh Responder JOIN matching
// that immutable tuple. Connection still owns expected generation, offsets,
// original secret, finite proposals and final physical/stream commit.
func (lease *ServiceLease) BeginReplacement(opening *Opening, physical *join.Joined) (*ServiceAttachment, error) {
	if lease == nil || lease.self != lease || lease.lifetime == nil || lease.lifetime.binding == nil || lease.lifetime.binding.root == nil || opening == nil || opening.recipient == nil || physical == nil {
		return nil, ErrUnavailable
	}
	if err := lease.CheckIdentity(lease.identity, lease.logical); err != nil {
		return nil, err
	}
	request, digest, err := opening.Request(lease.ctx)
	if err != nil {
		return nil, err
	}
	if request.AttachmentGeneration < 2 || !time.Now().Before(time.Unix(lease.identity.NoNewRecoveryAfter, 0)) || request.Deadline.After(time.Unix(lease.identity.NoNewRecoveryAfter, 0)) {
		return nil, ErrUnavailable
	}
	if err := physical.CheckResponderBinding(sha256.Sum256(request.JoinSecret[:]), request.HandshakeContext, time.Unix(lease.identity.WorkSafetyNotAfter, 0), request.Deadline); err != nil {
		return nil, err
	}
	if err := physical.CheckLifetime(time.Unix(lease.identity.WorkSafetyNotAfter, 0)); err != nil {
		return nil, err
	}
	root := lease.lifetime.binding.root
	root.mu.Lock()
	if err := lease.checkCommitLocked(); err != nil {
		root.mu.Unlock()
		return nil, err
	}
	if opening.recipient.binding != lease.lifetime.binding || lease.continuityCommitment == [32]byte{} {
		root.mu.Unlock()
		return nil, ErrUnavailable
	}
	value := lease.lifetime.binding.credential.Delegation()
	identity := wire.LogicalIdentity{Network: request.Network, Target: request.Target, Instance: value.Instance, InstanceGeneration: value.Generation,
		Publication: request.PublicationDigest, Profile: request.ProfileDigest, ConnectionNonce: request.ConnectionNonce, InitiatorBinding: request.InitiatorBinding,
		WorkSafetyNotAfter: request.WorkSafetyNotAfter, WorkSafetyMaximum: request.WorkSafetyMaximum, NoNewRecoveryAfter: request.NoNewRecoveryAfter}
	logical, attachment, err := wire.Contexts(identity, digest, request.AttachmentGeneration)
	if err != nil || identity != lease.identity || logical != lease.logical || !time.Now().Before(time.Unix(identity.NoNewRecoveryAfter, 0)) {
		root.mu.Unlock()
		return nil, errors.Join(ErrUnavailable, err)
	}
	// One setup borrow at a time; this is physical key-use confinement, not
	// Connection proposal accounting or logical-generation advancement.
	for retained := range lease.attachments {
		if !retained.closed && !retained.initialComplete {
			root.mu.Unlock()
			return nil, ErrUnavailable
		}
	}
	if !opening.serviceClaim.CompareAndSwap(false, true) {
		root.mu.Unlock()
		return nil, ErrUnavailable
	}
	child, cancel := context.WithCancel(lease.ctx)
	service := &ServiceAttachment{lifetime: lease.lifetime, lease: lease, opening: opening, physical: physical, ctx: child, cancel: cancel,
		identity: identity, logical: logical, attachment: attachment, generation: request.AttachmentGeneration, setupDeadline: request.Deadline,
		setupContext: opening.ctx, lifetimeCallback: make(chan struct{}), watcherDone: make(chan struct{})}
	service.self = service
	service.stopLifetime = context.AfterFunc(lease.lifetime.ctx, func() { defer close(service.lifetimeCallback); cancel() })
	err = service.configureTLSLocked()
	if err != nil {
		cancel()
		root.mu.Unlock()
		if !service.stopLifetime() {
			<-service.lifetimeCallback
		}
		return nil, err // Physical JOIN remains with the rejecting caller.
	}
	lease.attachments[service] = struct{}{}
	root.mu.Unlock()
	go service.watch()
	if _, _, err := opening.Request(lease.ctx); err != nil {
		return nil, errors.Join(err, service.Close())
	}
	if err := service.CheckIdentity(identity, logical, attachment); err != nil {
		return nil, errors.Join(err, service.Close())
	}
	return service, nil
}

// SendReplacement consumes a verified original-key response on this exact
// fresh TLS. The actual complete write is retained, but setup stays bounded
// and Application writes remain denied until Connection's final transfer.
func (service *ServiceAttachment) SendReplacement(ctx context.Context, response *wire.ReplacementResponse) error {
	if ctx == nil {
		return ErrUnavailable
	}
	if err := service.enter(); err != nil {
		return err
	}
	defer service.users.Done()
	root := service.lifetime.binding.root
	root.mu.Lock()
	if service.generation < 2 || service.initialComplete || service.responseStarted || service.evidence == nil || ctx.Err() != nil ||
		service.lease.continuityCommitment == [32]byte{} || !time.Now().Before(time.Unix(service.identity.NoNewRecoveryAfter, 0)) {
		root.mu.Unlock()
		return ErrUnavailable
	}
	body, err := response.Consume(service.evidence, wire.RolePublisher, service.generation, service.logical, service.lease.continuityCommitment)
	if err == nil {
		service.responseStarted = true
	}
	root.mu.Unlock()
	if err != nil {
		return err
	}
	defer clear(body)
	n, err := service.connection.Write(body)
	if n != len(body) {
		err = errors.Join(err, io.ErrShortWrite)
	}
	if err := errors.Join(err, ctx.Err(), service.recheck()); err != nil {
		return err
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := errors.Join(ctx.Err(), service.checkLocked()); err != nil {
		return err
	}
	if !time.Now().Before(time.Unix(service.identity.NoNewRecoveryAfter, 0)) {
		return ErrUnavailable
	}
	service.replacementWritten = true
	return nil
}

// TransferReplacement follows Connection's joined offset/generation decision
// and final scope checks. It consumes actual response-write state, never an
// established flag. Instance neither resets counters nor selects a generation.
func (service *ServiceAttachment) TransferReplacement(ctx context.Context) error {
	if ctx == nil {
		return ErrUnavailable
	}
	if err := service.enter(); err != nil {
		return err
	}
	defer service.users.Done()
	root := service.lifetime.binding.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if service.generation < 2 || service.initialComplete || !service.responseStarted || !service.replacementWritten || service.evidence == nil ||
		!time.Now().Before(time.Unix(service.identity.NoNewRecoveryAfter, 0)) {
		return ErrUnavailable
	}
	if err := errors.Join(ctx.Err(), service.checkLocked()); err != nil {
		return err
	}
	if err := service.physical.SetDeadline(time.Unix(service.identity.WorkSafetyNotAfter, 0)); err != nil {
		return err
	}
	if err := errors.Join(ctx.Err(), service.checkLocked()); err != nil {
		return err
	}
	service.initialComplete = true
	return nil
}
