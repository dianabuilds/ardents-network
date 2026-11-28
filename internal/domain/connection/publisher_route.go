package connection

import (
	"context"
	"crypto/rand"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/domain/route/join"
	"github.com/dianabuilds/ardents-network/internal/domain/route/prefix"
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
	return owner.openPublisherStream(contract, source, responder, nil, nil)
}

// OpenPublisherSegment admits a first segment or the exact next transition
// under the original live publication. Its independently admitted Attachment
// must authenticate the pending Resume before either record actor starts.
func (owner *AuthenticatedAttachment) OpenPublisherSegment(ctx context.Context, contract executionruntime.StreamContract, source *prefix.SourceScope, responder *prefix.ResponderScope, previous *Stream) (*Stream, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, errContinuationRefused
	}
	if previous != nil {
		claim, err := previous.publisherContinuation(ctx)
		if err != nil {
			return nil, err
		}
		return owner.openPublisherStream(contract, source, responder, nil, &segmentResume{publisher: claim})
	}
	var ticket [32]byte
	if _, err := rand.Read(ticket[:]); err != nil {
		return nil, err
	}
	segment, err := newSegmentBoundaryExchange(wire.RolePublisher, wire.SegmentBoundary{Next: 1}, ticket)
	if err != nil {
		return nil, err
	}
	return owner.openPublisherStream(contract, source, responder, segment, nil)
}

func (owner *AuthenticatedAttachment) openPublisherStream(contract executionruntime.StreamContract, source *prefix.SourceScope, responder *prefix.ResponderScope, segment *segmentBoundaryExchange, resume *segmentResume) (*Stream, error) {
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
	return owner.openStreamWithSegment(contract, route, nil, segment, resume)
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

func (route *publisherRouteAcquisition) checkRecipient(config join.Config) error {
	return route.handle.CheckRendezvous(config.Duty, nil)
}

func (route *publisherRouteAcquisition) join(ctx context.Context) (*join.Acquisition, error) {
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
