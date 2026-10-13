package connection

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
	"github.com/dianabuilds/ardents-network/internal/successor/route/join"
)

// AuthenticateInitiator requires an actual retained JOIN and independently
// bound Reader destination. Exact certificate, protocol proof and coalesced
// zero-offset Continuity all precede accepted initial state.
func AuthenticateInitiator(ctx context.Context, binding *InitiatorBinding, physical *join.Joined) (_ *AuthenticatedAttachment, result error) {
	if ctx == nil || binding == nil || binding.source == nil || physical == nil {
		return nil, errIntegrity
	}
	if err := errors.Join(ctx.Err(), binding.check(true), physical.CheckSourceBinding(binding.joinSecretDigest, binding.joinContext, time.Unix(binding.identity.WorkSafetyNotAfter, 0), binding.expires)); err != nil {
		return nil, err
	}
	logicalContext, cancelLogical, child, cancel := initiatorLifetimes(binding.operation.Context(), time.Unix(binding.identity.WorkSafetyNotAfter, 0))
	attachment := &initiatorAttachment{binding: binding, physical: physical, ctx: child, cancel: cancel, watcherDone: make(chan struct{})}
	attachment.self = attachment
	attachment.connection = tls.Client(initiatorTLSBorrow{attachment: attachment}, &tls.Config{
		MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519},
		InsecureSkipVerify: true, // The independently bound Instance is the sole verifier.
		VerifyConnection: func(state tls.ConnectionState) error {
			return verifyInstanceCertificate(state, binding.identity.Instance)
		},
	})
	owner := &AuthenticatedAttachment{role: wire.RoleInitiator, operation: binding.operation, initiator: attachment,
		identity: binding.identity, logical: binding.logical, attachment: binding.attachment,
		logicalContext: logicalContext, cancelLogical: cancelLogical, recipientRevision: binding.revision}
	owner.self = owner
	go attachment.watch()
	defer func() {
		if result != nil {
			result = errors.Join(result, owner.Close())
		}
	}()
	callerDone := make(chan struct{})
	stopCaller := context.AfterFunc(ctx, func() { defer close(callerDone); _ = owner.Close() })
	originalSetupDone := make(chan struct{})
	stopOriginalSetup := context.AfterFunc(binding.setup, func() { defer close(originalSetupDone); _ = owner.Close() })
	var callerOnce sync.Once
	joinCaller := func() {
		callerOnce.Do(func() {
			if !stopCaller() {
				<-callerDone
			}
			if !stopOriginalSetup() {
				<-originalSetupDone
			}
		})
	}
	defer joinCaller()
	if err := physical.SetDeadline(binding.expires); err != nil {
		return nil, err
	}
	if err := attachment.enter(); err != nil {
		return nil, err
	}
	err := attachment.connection.HandshakeContext(attachment.ctx)
	attachment.users.Done()
	if err := errors.Join(err, ctx.Err(), attachment.check()); err != nil {
		return nil, err
	}
	evidence, err := wire.BindTLS(attachment.connection, binding.attachment)
	if err != nil {
		return nil, err
	}
	attachment.mu.Lock()
	if err := attachment.checkLocked(); err != nil {
		attachment.mu.Unlock()
		evidence.Erase()
		return nil, err
	}
	attachment.evidence = evidence
	attachment.mu.Unlock()
	key, exporter := evidence.Secrets()
	defer clear(key[:])
	challenge := wire.Challenge{Network: binding.identity.Network, Target: binding.identity.Target, InstanceGeneration: binding.identity.InstanceGeneration, Context: binding.logical}
	if _, err := rand.Read(challenge.Nonce[:]); err != nil || challenge.Nonce == [32]byte{} {
		return nil, errIntegrity
	}
	request, err := wire.NewContinuity(key, wire.RoleInitiator, 1, 0, 0, 0, binding.logical, exporter)
	if err != nil || request.Nonce == challenge.Nonce {
		return nil, errors.Join(errIntegrity, err)
	}
	var flight bytes.Buffer
	if err := wire.Write(&flight, wire.Record{Challenge: &challenge}); err != nil {
		return nil, err
	}
	if err := wire.Write(&flight, wire.Record{Continuity: &request}); err != nil {
		return nil, err
	}
	n, err := attachment.Write(flight.Bytes())
	if err != nil || n != flight.Len() {
		return nil, errors.Join(err, io.ErrShortWrite)
	}
	first, err := wire.Read(attachment)
	if err != nil || first.Proof == nil {
		return nil, errors.Join(errIntegrity, err)
	}
	second, err := wire.Read(attachment)
	if err != nil || second.Continuity == nil {
		return nil, errors.Join(errIntegrity, err)
	}
	receipt, err := wire.VerifyPeerInitialResponse(evidence, binding.identity.Instance, challenge, request, *first.Proof, *second.Continuity)
	if err != nil {
		return nil, err
	}
	if err := errors.Join(ctx.Err(), binding.check(true), attachment.check()); err != nil {
		return nil, err
	}
	if err := receipt.Consume(evidence, challenge); err != nil {
		return nil, err
	}
	if err := physical.SetDeadline(time.Unix(binding.identity.WorkSafetyNotAfter, 0)); err != nil {
		return nil, err
	}
	attachment.mu.Lock()
	if err := attachment.checkLocked(); err != nil {
		attachment.mu.Unlock()
		return nil, err
	}
	attachment.initialComplete = true
	attachment.mu.Unlock()
	joinCaller()
	if err := errors.Join(ctx.Err(), binding.check(true), owner.check()); err != nil {
		return nil, err
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed {
		return nil, errIntegrity
	}
	owner.key, owner.exporter = key, exporter
	return owner, nil
}
