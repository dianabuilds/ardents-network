package connection

import (
	"context"
	"crypto/subtle"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

var errContinuationRefused = errors.New("service Connection continuation refused")

// continuationClaim owns one confirmed transition within the original
// transfer. The transfer coordinator retains at most one claim and must gate
// the new Connection's Application bytes until it confirms the binding.
type continuationClaim struct {
	ctx      context.Context
	previous *Stream
	boundary wire.SegmentBoundary
	ticket   [32]byte

	mu       sync.Mutex
	accepted [32]byte
}

func (previous *Stream) publisherContinuation(ctx context.Context) (*continuationClaim, error) {
	if previous == nil || previous.self != previous {
		return nil, errContinuationRefused
	}
	select {
	case <-previous.watcherDone:
	default:
		return nil, errContinuationRefused
	}
	if previous.JoinedCleanup() != nil || previous.closeErr != nil || previous.segmentBoundary == nil {
		return nil, errContinuationRefused
	}
	previous.mu.Lock()
	defer previous.mu.Unlock()
	if previous.pendingPublisher != nil {
		return previous.pendingPublisher, nil
	}
	boundary := previous.segmentBoundary
	claim, err := newContinuationClaim(ctx, previous, boundary.boundary, boundary.ticket)
	if err != nil {
		return nil, err
	}
	previous.pendingPublisher = claim
	return claim, nil
}

func newContinuationClaim(ctx context.Context, previous *Stream, boundary wire.SegmentBoundary, ticket [32]byte) (*continuationClaim, error) {
	if ctx == nil || ctx.Err() != nil || previous == nil || previous.self != previous ||
		previous.owner == nil || previous.owner.role != wire.RolePublisher || previous.owner.logical == [32]byte{} ||
		!boundary.Valid() || ticket == [32]byte{} {
		return nil, errContinuationRefused
	}
	return &continuationClaim{ctx: ctx, previous: previous, boundary: boundary, ticket: ticket}, nil
}

// bind verifies the new independently admitted original Attachment. This
// does not open a Route, choose a target, or consume the Attachment's stream.
func (claim *continuationClaim) bind(next *AuthenticatedAttachment, generation uint64, request wire.Continuation) (wire.Continuation, error) {
	if next == nil || next.role != wire.RolePublisher || next.operation != claim.previous.owner.operation ||
		generation == 0 || next.check() != nil {
		return wire.Continuation{}, errContinuationRefused
	}
	return claim.bindVerified(next.identity, next.logical, generation, request)
}

// bindVerified is the exact logical gate after the attachment's current
// authority was checked. Replaying the same response never authorizes a
// second logical Connection.
func (claim *continuationClaim) bindVerified(identity wire.LogicalIdentity, logical [32]byte, generation uint64, request wire.Continuation) (wire.Continuation, error) {
	claim.mu.Lock()
	defer claim.mu.Unlock()
	previous := claim.previous
	select {
	case <-previous.watcherDone:
		// Both the physical work and original retirement watcher must join.
	default:
		return wire.Continuation{}, errContinuationRefused
	}
	original := previous.owner
	retired := previous.segmentBoundary
	if claim.ctx.Err() != nil || previous.JoinedCleanup() != nil || previous.closeErr != nil ||
		retired == nil || retired.boundary != claim.boundary || retired.ticket != claim.ticket ||
		request.Kind != wire.ResumeTransfer || generation == 0 || request.AttachmentGeneration != generation ||
		request.Boundary != claim.boundary || logical == [32]byte{} || logical == original.logical ||
		request.LogicalContext != logical || !sameContinuationService(original.identity, identity) ||
		subtle.ConstantTimeCompare(request.Ticket[:], claim.ticket[:]) != 1 {
		return wire.Continuation{}, errContinuationRefused
	}
	if claim.accepted != [32]byte{} && claim.accepted != logical {
		return wire.Continuation{}, errContinuationRefused
	}
	claim.accepted = logical
	request.Kind = wire.AcceptResume
	return request, nil
}

func sameContinuationService(previous, next wire.LogicalIdentity) bool {
	return next.Network == previous.Network && next.Target == previous.Target &&
		next.Instance == previous.Instance && next.InstanceGeneration == previous.InstanceGeneration &&
		next.Publication == previous.Publication && next.Profile == previous.Profile
}
