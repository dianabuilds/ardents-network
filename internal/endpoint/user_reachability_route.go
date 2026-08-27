package endpoint

import (
	"context"
	"crypto/tls"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// UserReachabilityRouteRequest combines either one opaque private lookup or an
// already-obtained descriptor with independently State-selected User peers.
// The descriptor owns Publication and live Introduction-slot facts; callers
// cannot replace them with plan data.
type UserReachabilityRouteRequest struct {
	TargetLink                          string
	Descriptor                          []byte
	Private                             *UserPrivateReachabilityRequest
	Introduction, Initiator, Rendezvous TransitPeer
	Entry                               route.EntryAcquirer
	AttachmentID, EndpointHandshake     [32]byte
	// SubmissionAuthorization and SubmissionClientCertificate are produced
	// only by the membership credential owner for Descriptor v2. Descriptor v1
	// continues to use its embedded fixed Grant.
	SubmissionAuthorization     []byte
	SubmissionClientCertificate tls.Certificate
	At                          time.Time
}

// OpenUserReachabilityRoute performs one selected private lookup when present,
// verifies its descriptor before spending C-2 Entry work, checks its fixed
// Introduction/Rendezvous identities against State-selected peers, then invokes
// the existing bounded C-2 route composition. It performs no peer discovery.
func (endpoint *endpoint) OpenUserReachabilityRoute(ctx context.Context, input UserReachabilityRouteRequest) (*UserIntroductionRoute, error) {
	if endpoint == nil || ctx == nil || input.At.IsZero() || (len(input.Descriptor) == 0 && input.Private == nil) ||
		(len(input.Descriptor) != 0 && input.Private != nil) || !validTransitPeer(input.Introduction) ||
		!validTransitPeer(input.Initiator) || !validTransitPeer(input.Rendezvous) || input.Entry == nil ||
		input.AttachmentID == [32]byte{} || input.EndpointHandshake == [32]byte{} ||
		(input.Private != nil && (input.Private.AttachmentID == input.AttachmentID || input.Private.GatewayNodeID == input.Introduction.NodeID ||
			input.Private.GatewayNodeID == input.Initiator.NodeID || input.Private.GatewayNodeID == input.Rendezvous.NodeID ||
			input.Private.GatewayFamily == input.Introduction.Family || input.Private.GatewayFamily == input.Initiator.Family ||
			input.Private.GatewayFamily == input.Rendezvous.Family || input.Introduction.Family == [32]byte{} ||
			input.Initiator.Family == [32]byte{} || input.Rendezvous.Family == [32]byte{})) {
		return nil, errors.New("user reachability route input is incomplete")
	}
	target, err := endpoint.TargetFromLink(input.TargetLink)
	if err != nil {
		return nil, err
	}
	descriptor := input.Descriptor
	if input.Private != nil {
		descriptor, err = endpoint.ResolveUserReachability(ctx, input.TargetLink, *input.Private)
		if err != nil {
			return nil, err
		}
	}
	verified, err := reachability.Verify(descriptor, target, endpoint.network, input.At)
	if err != nil {
		return nil, errors.Join(errors.New("private reachability evidence is invalid"), err)
	}
	slot := verified.Descriptor.Introduction
	submissionAuthorization := slot.SubmissionAuthorization
	if slot.SubmissionMode == reachability.SubmissionMembershipGrant {
		if len(input.SubmissionAuthorization) == 0 {
			return nil, errors.New("membership reachability descriptor has no issued transit credential")
		}
		submissionAuthorization = append([]byte(nil), input.SubmissionAuthorization...)
	} else if slot.SubmissionMode != reachability.SubmissionFixedGrant || len(input.SubmissionAuthorization) != 0 {
		return nil, errors.New("reachability submission authorization mode is invalid")
	}
	if input.Introduction.NodeID != slot.IntroductionNodeID || input.Rendezvous.NodeID != slot.RendezvousNodeID ||
		input.Introduction.NodeID == input.Initiator.NodeID || input.Introduction.NodeID == input.Rendezvous.NodeID ||
		input.Initiator.NodeID == input.Rendezvous.NodeID {
		return nil, errors.New("private reachability State peers do not match descriptor")
	}
	return endpoint.OpenUserIntroductionRoute(ctx, UserIntroductionRouteRequest{TargetLink: input.TargetLink,
		Publication: verified.Current.Record, AuthorityPublic: verified.Descriptor.AuthorityPublic,
		Introduction: UserIntroductionProfile{NetworkID: endpoint.network, Digest: slot.StateDigest, Epoch: slot.Epoch,
			Introduction: input.Introduction, RendezvousNodeID: slot.RendezvousNodeID, Reachability: slot.Reachability,
			JoinHandle: slot.JoinHandle, NotAfter: slot.NotAfter, SubmissionAuthorization: submissionAuthorization,
			SubmissionClientCertificate: input.SubmissionClientCertificate},
		Entry: input.Entry, Initiator: input.Initiator, Rendezvous: input.Rendezvous, AttachmentID: input.AttachmentID,
		EndpointHandshake: input.EndpointHandshake, At: input.At})
}
