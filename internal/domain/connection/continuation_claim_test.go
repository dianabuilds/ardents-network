package connection

import (
	"context"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

func continuationFixture(t *testing.T) (*Stream, *continuationClaim, wire.Continuation) {
	t.Helper()
	previous := &Stream{
		joined: make(chan struct{}), watcherDone: make(chan struct{}),
		owner: &AuthenticatedAttachment{
			role: wire.RolePublisher, logical: [32]byte{1},
			identity: wire.LogicalIdentity{
				Network: [32]byte{2}, Target: [32]byte{3}, Instance: [32]byte{4}, InstanceGeneration: 5,
				Publication: [32]byte{6}, Profile: [32]byte{7},
			},
		},
	}
	previous.self = previous
	boundary := wire.SegmentBoundary{Next: 2, ToPublisher: 6, ToReader: 52, EOF: 1}
	ticket := [32]byte{9}
	claim, err := newContinuationClaim(t.Context(), previous, boundary, ticket)
	if err != nil {
		t.Fatal(err)
	}
	request := wire.Continuation{
		Kind: wire.ResumeTransfer, AttachmentGeneration: 1, Boundary: boundary, Ticket: ticket, LogicalContext: [32]byte{10},
	}
	return previous, claim, request
}

func TestContinuationClaimRequiresJoinedOriginalAndExactResume(t *testing.T) {
	previous, claim, request := continuationFixture(t)
	identity := previous.owner.identity
	attempt := func(value wire.Continuation, facts wire.LogicalIdentity) error {
		_, err := claim.bindVerified(facts, [32]byte{10}, 1, value)
		return err
	}
	close(previous.joined)
	if err := attempt(request, identity); !errors.Is(err, errContinuationRefused) {
		t.Fatalf("before retirement watcher join: %v", err)
	}
	close(previous.watcherDone)
	if err := attempt(request, identity); !errors.Is(err, errContinuationRefused) {
		t.Fatalf("ordinary joined closure resumed: %v", err)
	}
	previous.segmentBoundary = &retiredSegmentBoundary{boundary: request.Boundary, ticket: [32]byte{99}}
	if err := attempt(request, identity); !errors.Is(err, errContinuationRefused) {
		t.Fatalf("resume did not match the retired segment ticket: %v", err)
	}
	previous.segmentBoundary.ticket = request.Ticket
	previous.cleanupErr = errors.New("failed original physical cleanup")
	if err := attempt(request, identity); !errors.Is(err, errContinuationRefused) {
		t.Fatalf("original cleanup failure: %v", err)
	}
	previous.cleanupErr = nil
	for _, change := range []func(*wire.Continuation){
		func(r *wire.Continuation) { r.Kind = wire.ConfirmBoundary },
		func(r *wire.Continuation) { r.AttachmentGeneration++ },
		func(r *wire.Continuation) { r.Boundary.Next++ },
		func(r *wire.Continuation) { r.Boundary.ToPublisher++ },
		func(r *wire.Continuation) { r.Boundary.ToReader++ },
		func(r *wire.Continuation) { r.Boundary.EOF ^= 2 },
		func(r *wire.Continuation) { r.Ticket[0] ^= 1 },
		func(r *wire.Continuation) { r.LogicalContext[0] ^= 1 },
	} {
		invalid := request
		change(&invalid)
		if err := attempt(invalid, identity); !errors.Is(err, errContinuationRefused) {
			t.Fatalf("mismatched resume accepted: %v", err)
		}
	}
	for _, alter := range []func(*wire.LogicalIdentity){
		func(i *wire.LogicalIdentity) { i.Network[0] ^= 1 },
		func(i *wire.LogicalIdentity) { i.Target[0] ^= 1 },
		func(i *wire.LogicalIdentity) { i.Instance[0] ^= 1 },
		func(i *wire.LogicalIdentity) { i.InstanceGeneration++ },
		func(i *wire.LogicalIdentity) { i.Publication[0] ^= 1 },
		func(i *wire.LogicalIdentity) { i.Profile[0] ^= 1 },
	} {
		invalid := identity
		alter(&invalid)
		if err := attempt(request, invalid); !errors.Is(err, errContinuationRefused) {
			t.Fatalf("mismatched service accepted: %v", err)
		}
	}
	accepted := request
	accepted.Kind = wire.AcceptResume
	for range 2 {
		if got, err := claim.bindVerified(identity, request.LogicalContext, 1, request); err != nil || got != accepted {
			t.Fatalf("original response = %+v, %v", got, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	claim.ctx = ctx
	if err := attempt(request, identity); !errors.Is(err, errContinuationRefused) {
		t.Fatalf("canceled transfer resumed: %v", err)
	}
}

func TestContinuationClaimBindsExactlyOneLogicalConnection(t *testing.T) {
	previous, claim, request := continuationFixture(t)
	previous.segmentBoundary = &retiredSegmentBoundary{boundary: request.Boundary, ticket: request.Ticket}
	close(previous.joined)
	close(previous.watcherDone)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, context := range [][32]byte{{10}, {11}} {
		go func() {
			resume := request
			resume.LogicalContext = context
			<-start
			_, err := claim.bindVerified(previous.owner.identity, context, 1, resume)
			results <- err
		}()
	}
	close(start)
	first, second := <-results, <-results
	if (first == nil) == (second == nil) {
		t.Fatalf("wanted one binding, got %v and %v", first, second)
	}
}
