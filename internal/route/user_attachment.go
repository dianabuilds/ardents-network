package route

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

const userLookupWindow = 15 * time.Second

func (route *Route) openUserAttachment(ctx context.Context, intent Intent, release func() error) (*Attachment, error) {
	fail := func(cause error) (*Attachment, error) { return nil, errors.Join(cause, release()) }
	at := route.config.Clock().UTC()
	if at.IsZero() {
		return fail(errors.New("user Route clock is unavailable"))
	}
	deadline := at.Add(userLookupWindow)
	view, err := route.config.Current()
	if err != nil || view == nil {
		return fail(errors.New("current State resolution view is unavailable"))
	}
	epoch, available := view.Epoch(at, deadline)
	if !available || epoch.NetworkID != route.config.NetworkID {
		return fail(errors.New("current State resolution epoch is unavailable"))
	}
	gateway, available := view.Gateway(at, deadline)
	if !available {
		return fail(errors.New("state destination resolution Gateway is unavailable"))
	}
	profile, err := reachability.DecodeGatewayProfile(gateway.Profile)
	if err != nil || reachability.VerifyGatewayProfile(profile, route.config.NetworkID, gateway.NodeID, gateway.PublicKey, at, deadline) != nil {
		return fail(errors.New("state destination resolution Gateway profile is invalid"))
	}
	contact, err := route.config.Entry.Contact()
	if err != nil {
		return fail(errors.New("current User Entry contact is unavailable"))
	}
	initiator, err := userInitiator(view, contact, at, deadline)
	if err != nil {
		return fail(err)
	}
	lookupAttachment, err := newUserAttachmentID()
	if err != nil {
		return fail(err)
	}
	descriptor, err := route.resolvePrivateReachability(ctx, intent.Target, epoch, gateway, profile, initiator, lookupAttachment, at, deadline)
	if err != nil {
		return fail(err)
	}
	verified, err := reachability.Verify(descriptor, intent.Target, route.config.NetworkID, at)
	if err != nil || verified.Descriptor.Introduction.StateDigest != epoch.Digest || verified.Descriptor.Introduction.Epoch != epoch.Number {
		return fail(errors.New("private reachability descriptor is not current State evidence"))
	}
	slot := verified.Descriptor.Introduction
	introduction, err := userStatePeer(view, slot.IntroductionNodeID, "introduction", at, slot.NotAfter)
	if err != nil {
		return fail(err)
	}
	rendezvous, err := userStatePeer(view, slot.RendezvousNodeID, "rendezvous", at, slot.NotAfter)
	if err != nil {
		return fail(err)
	}
	initiator, err = userInitiator(view, contact, at, slot.NotAfter)
	if err != nil {
		return fail(err)
	}
	if !distinctUserPeers(gateway, initiator, introduction, rendezvous) {
		return fail(errors.New("state private lookup and C-2 peers overlap"))
	}
	attachmentID, authorization, certificate, finish, err := route.submissionCredential(ctx, view, epoch, initiator, introduction, slot, at, deadline)
	if err != nil {
		return fail(err)
	}
	handshake, err := newUserAttachmentID()
	if err != nil {
		if finish != nil {
			err = errors.Join(err, finish(false))
		}
		return fail(err)
	}
	connection, cleanup, evidence, presented, err := route.openIntroduction(ctx, intent.Target, verified, initiator, introduction, rendezvous,
		attachmentID, handshake, authorization, certificate, at)
	if finish != nil {
		finishErr := finish(presented)
		if err != nil {
			return fail(errors.Join(err, finishErr))
		}
		if finishErr != nil {
			return nil, errors.Join(finishErr, cleanup(), release())
		}
	} else if err != nil {
		return fail(err)
	}
	if err != nil {
		return fail(err)
	}
	return &Attachment{connection: connection, close: func() error { return errors.Join(cleanup(), release()) }, evidence: evidence}, nil
}

func (route *Route) submissionCredential(ctx context.Context, view StateView, epoch state.ResolutionEpoch, initiator, introduction TransitPeer,
	slot reachability.Introduction, at, lookupDeadline time.Time,
) ([32]byte, []byte, tls.Certificate, func(bool) error, error) {
	if slot.SubmissionMode == reachability.SubmissionFixedGrant {
		attachment, err := fixedSubmissionAttachment(slot.SubmissionAuthorization, epoch, introduction.NodeID, slot.NotAfter)
		if err != nil {
			return [32]byte{}, nil, tls.Certificate{}, nil, err
		}
		return attachment, append([]byte(nil), slot.SubmissionAuthorization...), tls.Certificate{}, nil, nil
	}
	if slot.SubmissionMode != reachability.SubmissionMembershipGrant || !at.Before(slot.NotAfter) || slot.NotAfter.After(lookupDeadline) {
		return [32]byte{}, nil, tls.Certificate{}, nil, errors.New("reachability descriptor has no valid membership credential window")
	}
	issuer, available := view.CredentialIssuer(at, slot.NotAfter)
	if !available || issuer.NodeID == initiator.NodeID || issuer.NodeID == introduction.NodeID ||
		issuer.Family == initiator.Family || issuer.Family == introduction.Family {
		return [32]byte{}, nil, tls.Certificate{}, nil, errors.New("current State transit issuer is unavailable or overlaps the Route")
	}
	attachment, err := newUserAttachmentID()
	if err != nil {
		return [32]byte{}, nil, tls.Certificate{}, nil, err
	}
	carrier, err := newUserAttachmentID()
	if err != nil {
		return [32]byte{}, nil, tls.Certificate{}, nil, err
	}
	result, err := route.config.Credentials(ctx, CredentialRequest{Epoch: epoch, Issuer: issuer, Initiator: initiator, Transit: introduction,
		TransitRole: IntroductionRole, AttachmentID: attachment, At: at, NotAfter: slot.NotAfter,
		Exchange: func(exchangeCtx context.Context, envelope []byte) ([]byte, error) {
			return route.exchangeCredential(exchangeCtx, epoch, initiator, issuer, carrier, slot.NotAfter, envelope)
		}})
	if err != nil {
		return [32]byte{}, nil, tls.Certificate{}, nil, err
	}
	if len(result.Authorization) == 0 || len(result.Authorization) > maximumTransitAuthorization || result.ClientCertificate.PrivateKey == nil ||
		result.ClientCertificate.Leaf == nil || result.Finish == nil {
		if result.Finish != nil {
			_ = result.Finish(false)
		}
		return [32]byte{}, nil, tls.Certificate{}, nil, errors.New("endpoint credential adapter returned an incomplete membership credential")
	}
	return attachment, append([]byte(nil), result.Authorization...), result.ClientCertificate, result.Finish, nil
}

func userInitiator(view StateView, contact entry.Candidate, at, deadline time.Time) (TransitPeer, error) {
	candidate, available := view.Candidate(contact.NodeID, at, deadline)
	if !available || candidate.Domain != "initiator" || candidate.PublicKey != contact.PublicKey || candidate.Endpoint != contact.Endpoint ||
		sha256.Sum256([]byte(candidate.Family)) != contact.FamilyID {
		return TransitPeer{}, errors.New("user Entry contact does not match current Initiator State")
	}
	return TransitPeer{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, Family: contact.FamilyID, Endpoint: candidate.Endpoint}, nil
}

func userStatePeer(view StateView, nodeID [32]byte, domain string, at, deadline time.Time) (TransitPeer, error) {
	candidate, available := view.Candidate(nodeID, at, deadline)
	if !available || candidate.Domain != domain || candidate.NodeID == [32]byte{} || candidate.PublicKey == [32]byte{} || candidate.Endpoint == "" {
		return TransitPeer{}, errors.New("current State C-2 peer is unavailable")
	}
	return TransitPeer{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, Family: sha256.Sum256([]byte(candidate.Family)), Endpoint: candidate.Endpoint}, nil
}

func distinctUserPeers(gateway state.DestinationResolutionGateway, peers ...TransitPeer) bool {
	if gateway.NodeID == [32]byte{} || gateway.Family == [32]byte{} || len(peers) != 3 {
		return false
	}
	identities, families := [][32]byte{gateway.NodeID}, [][32]byte{gateway.Family}
	for _, peer := range peers {
		if peer.NodeID == [32]byte{} || peer.PublicKey == [32]byte{} || peer.Family == [32]byte{} || peer.Endpoint == "" {
			return false
		}
		identities, families = append(identities, peer.NodeID), append(families, peer.Family)
	}
	for index := range identities {
		for prior := 0; prior < index; prior++ {
			if identities[index] == identities[prior] || families[index] == families[prior] {
				return false
			}
		}
	}
	return true
}

func fixedSubmissionAttachment(authorization []byte, epoch state.ResolutionEpoch, introduction [32]byte, notAfter time.Time) ([32]byte, error) {
	grant, err := DecodeTransitGrant(authorization)
	if err != nil {
		return [32]byte{}, err
	}
	var authority ed25519.PublicKey
	for _, candidate := range epoch.Authorities {
		if candidate.ID == grant.IssuerID {
			authority = ed25519.PublicKey(candidate.PublicKey[:])
			break
		}
	}
	if authority == nil {
		return [32]byte{}, errors.New("introduction Transit Grant issuer is absent from current State")
	}
	grant, err = VerifyTransitGrant(authorization, authority)
	if err != nil || grant.NetworkID != epoch.NetworkID || grant.Digest != epoch.Digest || grant.Epoch != epoch.Number ||
		grant.TransitRole != IntroductionRole || grant.TransitNodeID != introduction || grant.AttachmentID == [32]byte{} ||
		notAfter.IsZero() || notAfter.After(grant.NotAfter) {
		return [32]byte{}, errors.New("introduction Transit Grant does not bind the current User Route")
	}
	return grant.AttachmentID, nil
}

func newUserAttachmentID() ([32]byte, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil || value == [32]byte{} {
		return [32]byte{}, errors.New("user Route could not create an attachment identifier")
	}
	return value, nil
}
