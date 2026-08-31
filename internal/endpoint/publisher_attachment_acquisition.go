package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

type publisherAttachmentStateView interface {
	resolutionCandidateView
	transitCredentialIssuerView
	PublisherAttachment(time.Time, time.Time) (state.PublisherAttachment, bool)
}

type publisherTransitAcquisition struct {
	view               transitCredentialIssuerView
	epoch              state.ResolutionEpoch
	entry              ApplicationEntry
	initiator, transit TransitPeer
	role               byte
	slot               reachability.Introduction
	at, deadline       time.Time
}

type publisherAcquisitionPlan struct {
	epoch                               state.ResolutionEpoch
	entry                               ApplicationEntry
	initiator, introduction, rendezvous TransitPeer
	responder                           TransitPeer
	slot                                reachability.Introduction
	at, deadline                        time.Time
}

func (plan publisherAcquisitionPlan) requests(view transitCredentialIssuerView) [2]publisherTransitAcquisition {
	return [2]publisherTransitAcquisition{
		{view: view, epoch: plan.epoch, entry: plan.entry, initiator: plan.initiator, transit: plan.introduction,
			role: route.IntroductionRole, slot: plan.slot, at: plan.at, deadline: plan.deadline},
		{view: view, epoch: plan.epoch, entry: plan.entry, initiator: plan.initiator, transit: plan.responder,
			role: route.ResponderRole, slot: plan.slot, at: plan.at, deadline: plan.deadline},
	}
}

type publisherCredentialCompletions struct {
	introduction func(bool) error
	responder    func(bool) error
}

type acquiredPublisherProfile struct {
	profile     PublisherIntroductionProfile
	credentials publisherCredentialCompletions
}

func (endpoint *endpoint) configurePublisher(current func() (publisherAttachmentStateView, error), entry ApplicationEntry,
	binding *instance.Binding,
) error {
	if endpoint == nil || current == nil || entry == nil || binding == nil || endpoint.publications == nil || endpoint.transitAcquire == nil {
		return errors.New("publisher composition owners are unavailable")
	}
	credential := binding.Credential()
	endpoint.publisherMu.Lock()
	defer endpoint.publisherMu.Unlock()
	if endpoint.publisherBinding != nil || endpoint.publisherSession != nil || credential.AuthorityPublic != endpoint.authority ||
		credential.IntroductionHPKEPublic != endpoint.introduction {
		return errors.New("publisher composition conflicts with Endpoint ownership")
	}
	endpoint.publisherBinding = binding
	endpoint.publisherPrepare = func(ctx context.Context, at time.Time) (acquiredPublisherProfile, error) {
		view, err := current()
		if err != nil || view == nil {
			return acquiredPublisherProfile{}, errors.Join(errors.New("current State resolution view is unavailable"), err)
		}
		return endpoint.acquirePublisherProfile(ctx, view, entry, credential, at)
	}
	return nil
}

// acquirePublisherProfile turns one indivisible authenticated State
// projection into the two independent role-scoped Grant lifecycles required
// by Publisher start. The returned finish functions remain Endpoint-owned.
func (endpoint *endpoint) acquirePublisherProfile(ctx context.Context, view publisherAttachmentStateView, entry ApplicationEntry,
	credential publication.Credential, at time.Time,
) (acquiredPublisherProfile, error) {
	if endpoint == nil || ctx == nil || view == nil || entry == nil || at.IsZero() ||
		credential.NetworkID != endpoint.network || at.Unix() < credential.NotBefore || at.Unix() >= credential.NotAfter {
		return acquiredPublisherProfile{}, errors.New("publisher attachment acquisition input is unavailable")
	}
	plan, err := endpoint.planPublisherAcquisition(view, entry, credential, at)
	if err != nil {
		return acquiredPublisherProfile{}, err
	}
	requests := plan.requests(view)
	acquire := func(input publisherTransitAcquisition) (transitCredentialSubmission, error) {
		return endpoint.acquireTransitCredential(ctx, input.view, input.epoch, input.entry,
			input.initiator, input.transit, input.role, input.slot, input.at, input.deadline)
	}
	introductionSubmission, err := acquire(requests[0])
	if err != nil || !validPublisherSubmission(introductionSubmission) {
		return acquiredPublisherProfile{}, errors.Join(err, errors.New("publisher Introduction credential is unavailable"))
	}
	responderSubmission, err := acquire(requests[1])
	if err != nil || !validPublisherSubmission(responderSubmission) || responderSubmission.attachment == introductionSubmission.attachment {
		return acquiredPublisherProfile{}, errors.Join(err, introductionSubmission.finish(false),
			errors.New("publisher Responder credential is unavailable"))
	}
	reachabilityID, reachabilityErr := applicationAttachmentID()
	joinHandle, joinErr := applicationAttachmentID()
	if reachabilityErr != nil || joinErr != nil {
		return acquiredPublisherProfile{}, errors.Join(reachabilityErr, joinErr,
			introductionSubmission.finish(false), responderSubmission.finish(false))
	}
	profile := PublisherIntroductionProfile{NetworkID: plan.epoch.NetworkID, Digest: plan.epoch.Digest, Epoch: plan.epoch.Number,
		Introduction: plan.introduction, Rendezvous: plan.rendezvous, Responder: plan.responder,
		SlotAttachmentID: introductionSubmission.attachment, ResponderAttachmentID: responderSubmission.attachment,
		Reachability: reachabilityID, JoinHandle: joinHandle, NotAfter: plan.deadline,
		SlotAuthorization:      append([]byte(nil), introductionSubmission.authorization...),
		ResponderAuthorization: append([]byte(nil), responderSubmission.authorization...)}
	if !validPublisherIntroductionProfile(profile) {
		return acquiredPublisherProfile{}, errors.Join(introductionSubmission.finish(false),
			responderSubmission.finish(false), errors.New("acquired Publisher profile is invalid"))
	}
	return acquiredPublisherProfile{profile: profile, credentials: publisherCredentialCompletions{
		introduction: introductionSubmission.finish, responder: responderSubmission.finish}}, nil
}

func (endpoint *endpoint) planPublisherAcquisition(view publisherAttachmentStateView, entry ApplicationEntry,
	credential publication.Credential, at time.Time,
) (publisherAcquisitionPlan, error) {
	deadline := at.UTC().Add(15 * time.Second).Truncate(time.Second)
	credentialDeadline := time.Unix(credential.NotAfter, 0).UTC()
	if credentialDeadline.Before(deadline) {
		deadline = credentialDeadline
	}
	if !at.Before(deadline) {
		return publisherAcquisitionPlan{}, errors.New("publisher attachment acquisition window is unavailable")
	}
	epoch, available := view.Epoch(at, deadline)
	attachment, attachmentAvailable := view.PublisherAttachment(at, deadline)
	if !available || !attachmentAvailable || epoch.NetworkID != endpoint.network || attachment.NetworkID != epoch.NetworkID ||
		attachment.Digest != epoch.Digest || attachment.Epoch != epoch.Number || !attachment.NotAfter.Equal(deadline) {
		return publisherAcquisitionPlan{}, errors.New("current State Publisher attachment is unavailable")
	}
	contact, err := entry.Contact()
	if err != nil {
		return publisherAcquisitionPlan{}, errors.New("current Publisher Entry contact is unavailable")
	}
	initiator, err := applicationInitiator(view, contact, at, deadline)
	if err != nil {
		return publisherAcquisitionPlan{}, err
	}
	introduction := publisherTransitPeer(attachment.Introduction)
	rendezvous := publisherTransitPeer(attachment.Rendezvous)
	responder := publisherTransitPeer(attachment.Responder)
	if !distinctPublisherTransitPeers(initiator, introduction, rendezvous, responder) {
		return publisherAcquisitionPlan{}, errors.New("state Publisher attachment roles overlap")
	}
	slot := reachability.Introduction{StateDigest: epoch.Digest, Epoch: epoch.Number,
		IntroductionNodeID: introduction.NodeID, RendezvousNodeID: rendezvous.NodeID, NotAfter: deadline,
		SubmissionMode: reachability.SubmissionMembershipGrant}
	return publisherAcquisitionPlan{epoch: epoch, entry: entry, initiator: initiator, introduction: introduction,
		rendezvous: rendezvous, responder: responder, slot: slot, at: at, deadline: deadline}, nil
}

func publisherTransitPeer(value state.PublisherTransitPeer) TransitPeer {
	return TransitPeer{NodeID: value.NodeID, PublicKey: value.PublicKey, Family: value.Family, Endpoint: value.Endpoint}
}

func distinctPublisherTransitPeers(peers ...TransitPeer) bool {
	if len(peers) != 4 {
		return false
	}
	for index, peer := range peers {
		if !validTransitPeer(peer) || peer.Family == [32]byte{} {
			return false
		}
		for prior := 0; prior < index; prior++ {
			if peer.NodeID == peers[prior].NodeID || peer.Family == peers[prior].Family {
				return false
			}
		}
	}
	return true
}

func validPublisherSubmission(value transitCredentialSubmission) bool {
	return value.attachment != [32]byte{} && len(value.authorization) > 0 && len(value.authorization) <= 1024 && value.finish != nil
}
