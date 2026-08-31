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
	ApplicationStateView
	PublisherAttachment(time.Time, time.Time) (state.PublisherAttachment, bool)
}

type publisherTransitAcquisition struct {
	view               ApplicationStateView
	epoch              state.ResolutionEpoch
	entry              ApplicationEntry
	initiator, transit TransitPeer
	role               byte
	slot               reachability.Introduction
	at, deadline       time.Time
}

type publisherTransitAcquirer func(context.Context, publisherTransitAcquisition) (transitCredentialSubmission, error)

func (endpoint *endpoint) configurePublisher(current func() (ApplicationStateView, error), entry ApplicationEntry,
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
	endpoint.publisherPrepare = func(ctx context.Context, at time.Time) (PublisherIntroductionProfile, func(bool) error, func(bool) error, error) {
		view, err := current()
		if err != nil || view == nil {
			return PublisherIntroductionProfile{}, nil, nil, errors.Join(errors.New("current State resolution view is unavailable"), err)
		}
		return endpoint.acquirePublisherProfile(ctx, view, entry, credential, at,
			func(acquireContext context.Context, input publisherTransitAcquisition) (transitCredentialSubmission, error) {
				return endpoint.acquireTransitCredential(acquireContext, input.view, input.epoch, input.entry,
					input.initiator, input.transit, input.role, input.slot, input.at, input.deadline)
			})
	}
	return nil
}

// acquirePublisherProfile turns one indivisible authenticated State
// projection into the two independent role-scoped Grant lifecycles required
// by Publisher start. The returned finish functions remain Endpoint-owned.
func (endpoint *endpoint) acquirePublisherProfile(ctx context.Context, current ApplicationStateView, entry ApplicationEntry,
	credential publication.Credential, at time.Time, acquire publisherTransitAcquirer,
) (PublisherIntroductionProfile, func(bool) error, func(bool) error, error) {
	view, projected := current.(publisherAttachmentStateView)
	if endpoint == nil || ctx == nil || !projected || entry == nil || acquire == nil || at.IsZero() ||
		credential.NetworkID != endpoint.network || at.Unix() < credential.NotBefore || at.Unix() >= credential.NotAfter {
		return PublisherIntroductionProfile{}, nil, nil, errors.New("publisher attachment acquisition input is unavailable")
	}
	deadline := at.UTC().Add(15 * time.Second).Truncate(time.Second)
	credentialDeadline := time.Unix(credential.NotAfter, 0).UTC()
	if credentialDeadline.Before(deadline) {
		deadline = credentialDeadline
	}
	if !at.Before(deadline) {
		return PublisherIntroductionProfile{}, nil, nil, errors.New("publisher attachment acquisition window is unavailable")
	}
	epoch, available := view.Epoch(at, deadline)
	attachment, attachmentAvailable := view.PublisherAttachment(at, deadline)
	if !available || !attachmentAvailable || epoch.NetworkID != endpoint.network || attachment.NetworkID != epoch.NetworkID ||
		attachment.Digest != epoch.Digest || attachment.Epoch != epoch.Number || !attachment.NotAfter.Equal(deadline) {
		return PublisherIntroductionProfile{}, nil, nil, errors.New("current State Publisher attachment is unavailable")
	}
	contact, err := entry.Contact()
	if err != nil {
		return PublisherIntroductionProfile{}, nil, nil, errors.New("current Publisher Entry contact is unavailable")
	}
	initiator, err := applicationInitiator(view, contact, at, deadline)
	if err != nil {
		return PublisherIntroductionProfile{}, nil, nil, err
	}
	introduction := publisherTransitPeer(attachment.Introduction)
	rendezvous := publisherTransitPeer(attachment.Rendezvous)
	responder := publisherTransitPeer(attachment.Responder)
	if !distinctPublisherTransitPeers(initiator, introduction, rendezvous, responder) {
		return PublisherIntroductionProfile{}, nil, nil, errors.New("state Publisher attachment roles overlap")
	}
	slot := reachability.Introduction{StateDigest: epoch.Digest, Epoch: epoch.Number,
		IntroductionNodeID: introduction.NodeID, RendezvousNodeID: rendezvous.NodeID, NotAfter: deadline,
		SubmissionMode: reachability.SubmissionMembershipGrant}
	introductionSubmission, err := acquire(ctx, publisherTransitAcquisition{view: view, epoch: epoch, entry: entry,
		initiator: initiator, transit: introduction, role: route.IntroductionRole, slot: slot, at: at, deadline: deadline})
	if err != nil || !validPublisherSubmission(introductionSubmission) {
		return PublisherIntroductionProfile{}, nil, nil, errors.Join(err, errors.New("publisher Introduction credential is unavailable"))
	}
	responderSubmission, err := acquire(ctx, publisherTransitAcquisition{view: view, epoch: epoch, entry: entry,
		initiator: initiator, transit: responder, role: route.ResponderRole, slot: slot, at: at, deadline: deadline})
	if err != nil || !validPublisherSubmission(responderSubmission) || responderSubmission.attachment == introductionSubmission.attachment {
		return PublisherIntroductionProfile{}, nil, nil, errors.Join(err, introductionSubmission.finish(false),
			errors.New("publisher Responder credential is unavailable"))
	}
	reachabilityID, reachabilityErr := applicationAttachmentID()
	joinHandle, joinErr := applicationAttachmentID()
	if reachabilityErr != nil || joinErr != nil {
		return PublisherIntroductionProfile{}, nil, nil, errors.Join(reachabilityErr, joinErr,
			introductionSubmission.finish(false), responderSubmission.finish(false))
	}
	profile := PublisherIntroductionProfile{NetworkID: epoch.NetworkID, Digest: epoch.Digest, Epoch: epoch.Number,
		Introduction: introduction, Rendezvous: rendezvous, Responder: responder,
		SlotAttachmentID: introductionSubmission.attachment, ResponderAttachmentID: responderSubmission.attachment,
		Reachability: reachabilityID, JoinHandle: joinHandle, NotAfter: deadline,
		SlotAuthorization:      append([]byte(nil), introductionSubmission.authorization...),
		ResponderAuthorization: append([]byte(nil), responderSubmission.authorization...)}
	if !validPublisherIntroductionProfile(profile) {
		return PublisherIntroductionProfile{}, nil, nil, errors.Join(introductionSubmission.finish(false),
			responderSubmission.finish(false), errors.New("acquired Publisher profile is invalid"))
	}
	return profile, introductionSubmission.finish, responderSubmission.finish, nil
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
