package connection

import (
	"context"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/publication/instance"
	"github.com/dianabuilds/ardents-network/internal/successor/route/join"
)

// AuthenticatePublisher executes the coalesced request/response over an actual
// JOIN and non-exporting original Instance. Setup cancellation is joined and
// detached at accepted transfer; the original operation and bounds remain.
func AuthenticatePublisher(ctx context.Context, operation *executionruntime.Operation, binding *RecipientBinding,
	lifetime *instance.PublicationLifetime, opening *instance.Opening, physical *join.Joined) (_ *AuthenticatedAttachment, result error) {
	if binding == nil || ctx == nil || operation == nil {
		return nil, errIntegrity
	}
	if err := binding.Check(ctx, operation); err != nil {
		return nil, err
	}
	lease, service, err := lifetime.BeginService(operation, opening, physical)
	if err != nil {
		return nil, err
	}
	logicalContext, cancelLogical := context.WithCancel(lease.Context())
	owner := &AuthenticatedAttachment{role: wire.RolePublisher, operation: operation, service: service, lease: lease, identity: binding.identity, logical: binding.logical, attachment: binding.attachment, recipientRevision: binding.revision, publisherJoin: physical,
		logicalContext: logicalContext, cancelLogical: cancelLogical}
	owner.self = owner
	defer func() {
		if result != nil {
			result = errors.Join(result, owner.Close())
		}
	}()
	callerDone := make(chan struct{})
	stopCaller := context.AfterFunc(ctx, func() { defer close(callerDone); _ = owner.Close() })
	var callerOnce sync.Once
	joinCaller := func() {
		callerOnce.Do(func() {
			if !stopCaller() {
				<-callerDone
			}
		})
	}
	defer joinCaller()
	check := func() error {
		if err := binding.Check(ctx, operation); err != nil {
			return err
		}
		if _, _, err := opening.Request(ctx); err != nil {
			return err
		}
		// BeginService constructs both handles together. Service identity
		// checking fully observes its same original lease, then matches this
		// immutable logical and Attachment tuple before returning.
		return service.CheckIdentity(binding.identity, binding.logical, binding.attachment)
	}
	if err := check(); err != nil {
		return nil, err
	}
	if err := service.Handshake(ctx); err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	evidence, err := service.Evidence()
	if err != nil {
		return nil, err
	}
	flight := initialFlightReader(service, wire.InitialRequestBytes)
	first, err := flight.readRecord()
	if err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	if first.Challenge == nil {
		return nil, errIntegrity
	}
	second, err := flight.readRecord()
	if err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	if second.Continuity == nil {
		return nil, errIntegrity
	}
	expected := wire.Challenge{Network: binding.identity.Network, Target: binding.identity.Target, InstanceGeneration: binding.identity.InstanceGeneration, Context: binding.logical}
	receipt, err := wire.VerifyInitialRequest(evidence, expected, *first.Challenge, *second.Continuity)
	if err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	proof, err := service.ProveInitial(ctx, receipt)
	if err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	key, exporter := evidence.Secrets()
	defer clear(key[:])
	continuity, err := wire.NewContinuity(key, wire.RolePublisher, 1, 0, 0, 0, binding.logical, exporter)
	if err != nil {
		return nil, err
	}
	response, err := wire.NewInitialResponse(receipt, binding.identity.Instance, proof, continuity)
	if err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	if err := service.SendInitial(ctx, response); err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	joinCaller()
	if err := errors.Join(ctx.Err(), operation.CheckPublisher(), service.CheckIdentity(binding.identity, binding.logical, binding.attachment)); err != nil {
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
