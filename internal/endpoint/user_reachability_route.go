package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// UserReachabilityRouteRequest combines one verified Target descriptor with
// independently State-selected User peers. Descriptor owns Publication and
// live Introduction-slot facts; the caller cannot replace them with plan data.
type UserReachabilityRouteRequest struct {
	TargetLink                          string
	Descriptor                          []byte
	Introduction, Initiator, Rendezvous TransitPeer
	Entry                               route.EntryAcquirer
	AttachmentID, EndpointHandshake     [32]byte
	At                                  time.Time
}

// OpenUserReachabilityRoute verifies one descriptor before spending Entry
// work, checks its fixed Introduction/Rendezvous identities against the
// caller's State-selected peers, then invokes the existing bounded C-2 route
// composition. It performs neither private lookup nor peer discovery.
func (endpoint *endpoint) OpenUserReachabilityRoute(ctx context.Context, input UserReachabilityRouteRequest) (*UserIntroductionRoute, error) {
	if endpoint == nil || ctx == nil || input.At.IsZero() || !validTransitPeer(input.Introduction) ||
		!validTransitPeer(input.Initiator) || !validTransitPeer(input.Rendezvous) || input.Entry == nil ||
		input.AttachmentID == [32]byte{} || input.EndpointHandshake == [32]byte{} {
		return nil, errors.New("User reachability route input is incomplete")
	}
	target, err := endpoint.TargetFromLink(input.TargetLink)
	if err != nil {
		return nil, err
	}
	verified, err := reachability.Verify(input.Descriptor, target, endpoint.network, input.At)
	if err != nil {
		return nil, errors.Join(errors.New("private reachability evidence is invalid"), err)
	}
	slot := verified.Descriptor.Introduction
	if input.Introduction.NodeID != slot.IntroductionNodeID || input.Rendezvous.NodeID != slot.RendezvousNodeID ||
		input.Introduction.NodeID == input.Initiator.NodeID || input.Introduction.NodeID == input.Rendezvous.NodeID ||
		input.Initiator.NodeID == input.Rendezvous.NodeID {
		return nil, errors.New("private reachability State peers do not match descriptor")
	}
	return endpoint.OpenUserIntroductionRoute(ctx, UserIntroductionRouteRequest{TargetLink: input.TargetLink,
		Publication: verified.Current.Record, AuthorityPublic: verified.Descriptor.AuthorityPublic,
		Introduction: UserIntroductionProfile{NetworkID: endpoint.network, Digest: slot.StateDigest, Epoch: slot.Epoch,
			Introduction: input.Introduction, RendezvousNodeID: slot.RendezvousNodeID, Reachability: slot.Reachability,
			JoinHandle: slot.JoinHandle, NotAfter: slot.NotAfter, SubmissionAuthorization: slot.SubmissionAuthorization},
		Entry: input.Entry, Initiator: input.Initiator, Rendezvous: input.Rendezvous, AttachmentID: input.AttachmentID,
		EndpointHandshake: input.EndpointHandshake, At: input.At})
}
