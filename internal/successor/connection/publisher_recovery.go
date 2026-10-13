package connection

import (
	"errors"
	"time"

	"context"
	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/route/join"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
)

// Publication retains these actual prefixes and owns incoming dispatch.
// Connection owns the finite episode and its authenticated logical commit.
// No accepting callback or copied physical provenance constructs this source.
type publisherRecoveryRoute struct {
	source    *prefix.SourceScope
	responder *prefix.ResponderScope
}

func (route *publisherRecoveryRoute) check(owner *AuthenticatedAttachment) error {
	if route == nil {
		return nil
	}
	if owner == nil || owner.self != owner || owner.role != wire.RolePublisher || owner.operation == nil || route.source == nil || route.responder == nil {
		return errIntegrity
	}
	end := time.Unix(owner.identity.WorkSafetyMaximum, 0).UTC()
	return errors.Join(route.source.CheckSourceCaller(owner.operation.Context(), owner.identity.Network, owner.identity.Profile, end), route.responder.CheckSource(route.source, end))
}

// OpenPublisherStream retains genuine original replacement Route owners before
// starting record actors. Publication must separately route private replacement
// openings to this exact stream; this constructor grants no incoming acceptance.
func (owner *AuthenticatedAttachment) OpenPublisherStream(contract executionruntime.StreamContract, source *prefix.SourceScope, responder *prefix.ResponderScope) (*Stream, error) {
	route := &publisherRecoveryRoute{source: source, responder: responder}
	if err := route.check(owner); err != nil {
		return nil, err
	}
	if owner.publisherJoin == nil {
		return nil, errIntegrity
	}
	if err := owner.publisherJoin.CheckResponderScope(responder); err != nil {
		return nil, err
	}
	if err := responder.Commit(func(checkOriginal func() error) error { return checkOriginal() }); err != nil {
		return nil, err
	}
	return owner.openStream(contract, route, nil)
}

// The physical handle is acquired only after Connection reserves its finite
// proposal. It cannot select a new Source/leg or follow another generation.
type publisherRouteAcquisition struct{ handle *prefix.ResponderHandle }

func (route *publisherRecoveryRoute) acquire(ctx context.Context) (*publisherRouteAcquisition, error) {
	handle, err := route.responder.Acquire(ctx)
	if err != nil {
		// Acquisition can join a failed original Responder before its fresh
		// preparation refuses. Keep that original outcome with the refusal;
		// no replacement handle exists to carry it into the later handoff.
		return nil, errors.Join(err, route.responder.RetiredWork())
	}
	return &publisherRouteAcquisition{handle: handle}, nil
}

func (route *publisherRouteAcquisition) checkRecipient(config join.JoinConfig) error {
	return route.handle.CheckRendezvous(config.Duty, nil)
}

func (route *publisherRouteAcquisition) join(ctx context.Context) (*join.JoinAcquisition, error) {
	return join.AcquireScopedResponderJoin(ctx, route.handle)
}

func (route *publisherRouteAcquisition) commit(action func(func() error) error) error {
	return route.handle.Commit(action)
}
func (route *publisherRouteAcquisition) close() {
	if route != nil {
		route.handle.Close()
	}
}

// Stream.mu admits one original interruption/join and clock for both physical
// actors, including simultaneous losses and peer proposals before local loss.
func (stream *Stream) startRecoveryLocked(records *attachmentRecords) error {
	if stream.records != records || records.generation != stream.send.generation || (stream.publisherRoute == nil && stream.initiatorRoute == nil) {
		return errIntegrity
	}
	if err := stream.checkLocked(); err != nil {
		return err
	}
	if records.retiring {
		return nil
	}
	if stream.recovery == nil {
		var err error
		stream.recovery, err = newRecoveryEpisode(records.generation, stream.lastProgress, time.Unix(stream.owner.identity.NoNewRecoveryAfter, 0))
		if err != nil {
			return err
		}
	}
	if err := stream.recovery.start(records.generation, time.Now()); err != nil {
		return err
	}
	records.retiring = true
	deadline := stream.recovery.deadline
	stream.recoveryUsers.Add(1)
	stream.signalLocked()
	go stream.joinFailedAttachment(records, deadline)
	if stream.initiatorRoute != nil {
		stream.recoveryUsers.Add(1)
		go stream.recoverInitiator(records)
	}
	return nil
}

func (stream *Stream) joinFailedAttachment(records *attachmentRecords, deadline time.Time) {
	defer stream.recoveryUsers.Done()
	go func() {
		var work, physical error
		if records.physical.initiator != nil {
			work = records.physical.initiator.closeFailed()
			physical = records.physical.initiator.cleanupErr
		} else {
			work = records.physical.service.CloseFailed()
			var joined bool
			physical, joined = records.physical.service.CleanupResult()
			if !joined {
				physical = errors.Join(physical, errIntegrity)
			}
		}
		work = errors.Join(work, records.join())
		stream.owner.operation.RetainCleanup(physical)
		stream.mu.Lock()
		if stream.recoveryLoss == nil {
			stream.recoveryLoss = work
		}
		stream.recoveryCleanup = errors.Join(stream.recoveryCleanup, physical)
		close(records.retired)
		stream.mu.Unlock()
	}()
	// A bound expires independently of a blocked physical join. Deny logical
	// effects on time, but retain the actor until that exact join really returns.
	defer func() { <-records.retired }()
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case <-stream.ctx.Done():
	case <-records.replaced:
	case <-timer.C:
		stream.mu.Lock()
		if stream.records == records && !stream.closing {
			stream.failLocked(errRecoveryExhausted)
		}
		stream.mu.Unlock()
	}
}
