package endpoint

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// alphaBrowserCredentialIssuerView is deliberately additional to the stable
// Alpha Browser Entry State view: Descriptor v1 must continue its fixed-Grant path, while a
// Descriptor v2 fails closed if the current State cannot project this new
// exact issuer duty.
type alphaBrowserCredentialIssuerView interface {
	CredentialIssuer(time.Time, time.Time) (state.TransitIssuer, bool)
}

type alphaBrowserSubmission struct {
	authorization []byte
	attachment    [32]byte
	certificate   tls.Certificate
	erase         func()
}

func (endpoint *endpoint) alphaBrowserSubmission(ctx context.Context, view AlphaBrowserStateView, epoch state.ResolutionEpoch,
	entry AlphaBrowserEntry, initiator, introduction TransitPeer, slot reachability.Introduction, at, deadline time.Time) (alphaBrowserSubmission, error) {
	if entry == nil {
		return alphaBrowserSubmission{}, errors.New("credential relay Entry owner is unavailable")
	}
	if slot.SubmissionMode == reachability.SubmissionFixedGrant {
		attachment, err := alphaBrowserServiceAttachment(slot.SubmissionAuthorization, epoch, introduction.NodeID, slot.NotAfter)
		if err != nil {
			return alphaBrowserSubmission{}, err
		}
		// Descriptor v1 already carries its fixed Grant.  Do not hand it back
		// through the v2-only caller input: OpenUserReachabilityRoute takes the
		// exact descriptor field for this compatibility path.
		return alphaBrowserSubmission{attachment: attachment}, nil
	}
	if slot.SubmissionMode != reachability.SubmissionMembershipGrant {
		return alphaBrowserSubmission{}, errors.New("reachability descriptor submission mode is unsupported")
	}
	issuerView, available := view.(alphaBrowserCredentialIssuerView)
	if !available {
		return alphaBrowserSubmission{}, errors.New("current State does not project a transit issuer")
	}
	issuer, available := issuerView.CredentialIssuer(at, deadline)
	if !available || issuer.NodeID == initiator.NodeID || issuer.NodeID == introduction.NodeID || issuer.Family == initiator.Family || issuer.Family == introduction.Family {
		return alphaBrowserSubmission{}, errors.New("current State transit issuer is unavailable or overlaps the route")
	}
	profile, err := credential.DecodeProfile(issuer.Profile)
	if err != nil || credential.VerifyProfile(profile, endpoint.network, issuer.NodeID, issuer.PublicKey, at, deadline) != nil {
		return alphaBrowserSubmission{}, errors.New("current State transit issuer profile is invalid")
	}
	serviceAttachment, err := alphaBrowserRandomID()
	if err != nil {
		return alphaBrowserSubmission{}, err
	}
	carrierAttachment, err := alphaBrowserRandomID()
	if err != nil || carrierAttachment == serviceAttachment {
		return alphaBrowserSubmission{}, errors.New("credential relay attachment is unavailable")
	}
	certificate, err := route.NewClientCertificate()
	if err != nil {
		return alphaBrowserSubmission{}, errors.New("transit client key is unavailable")
	}
	keyDigest, err := route.ClientTLSKeyDigest(certificate.Leaf)
	if err != nil {
		return alphaBrowserSubmission{}, errors.New("transit client key is invalid")
	}
	requestID, err := alphaBrowserRandomID()
	if err != nil {
		return alphaBrowserSubmission{}, errors.New("transit credential Request ID is unavailable")
	}
	client, err := credential.OpenClient(credential.ClientConfig{NetworkID: endpoint.network, IssuerPublic: issuer.PublicKey, Profile: profile,
		At: at, Deadline: deadline, Exchange: func(exchangeCtx context.Context, envelope []byte) ([]byte, error) {
			return endpoint.exchangeAlphaBrowserCredential(exchangeCtx, entry, epoch, initiator, issuer, carrierAttachment, deadline, envelope)
		}})
	if err != nil {
		return alphaBrowserSubmission{}, errors.New("transit issuer is unavailable")
	}
	result, err := client.Issue(ctx, credential.Request{RequestID: requestID, NetworkID: endpoint.network, Digest: epoch.Digest, Epoch: epoch.Number,
		IntroductionNodeID: introduction.NodeID, AttachmentID: serviceAttachment, ClientKeyDigest: keyDigest, NotAfter: deadline})
	if err != nil {
		return alphaBrowserSubmission{}, errors.New("membership transit credential is unavailable")
	}
	if result.Outcome != credential.Issued {
		return alphaBrowserSubmission{}, errors.New("membership transit credential is " + string(result.Outcome))
	}
	erase, err := endpoint.enrollTransitClient(result.Grant, certificate)
	if err != nil {
		return alphaBrowserSubmission{}, err
	}
	return alphaBrowserSubmission{authorization: result.Grant, attachment: serviceAttachment, certificate: certificate, erase: erase}, nil
}

func (endpoint *endpoint) exchangeAlphaBrowserCredential(ctx context.Context, source route.EntryAcquirer, epoch state.ResolutionEpoch,
	initiator TransitPeer, issuer state.TransitIssuer, attachment [32]byte, deadline time.Time, envelope []byte) ([]byte, error) {
	if endpoint == nil || ctx == nil || source == nil || epoch.NetworkID != endpoint.network || epoch.Digest == [32]byte{} || epoch.Number == 0 ||
		!validTransitPeer(initiator) || issuer.NodeID == [32]byte{} || issuer.PublicKey == [32]byte{} || attachment == [32]byte{} ||
		deadline.IsZero() || len(envelope) == 0 || len(envelope) > route.CredentialEnvelopeCapacity {
		return nil, errors.New("credential relay exchange is invalid")
	}
	connection, cleanup, err := route.OpenEntryAttachment(ctx, source, route.EntryAttachmentRequest{NetworkID: endpoint.network,
		Digest: epoch.Digest, Epoch: epoch.Number, AttachmentID: attachment, Deadline: deadline})
	if err != nil || connection == nil || cleanup == nil {
		if connection != nil {
			_ = connection.Close()
		}
		return nil, errors.New("credential relay Entry is unavailable")
	}
	defer cleanup()
	setup := route.CredentialRelaySetup{NetworkID: endpoint.network, Digest: epoch.Digest, Epoch: epoch.Number, AttachmentID: attachment,
		InitiatorNodeID: initiator.NodeID, IssuerNodeID: issuer.NodeID, IssuerNodePublicKey: issuer.PublicKey,
		IssuerProfileDigest: sha256.Sum256(issuer.Profile), NotAfter: deadline, EnvelopeCapacity: route.CredentialEnvelopeCapacity}
	if err := route.WriteCredentialRelaySetup(connection, setup); err != nil {
		return nil, errors.New("credential relay Initiator setup is unavailable")
	}
	ready, err := route.ReadCredentialRelayReady(connection)
	if err != nil || setup.VerifyCredentialRelayReady(ready) != nil {
		return nil, errors.New("credential relay Initiator confirmation is invalid")
	}
	if err := route.WriteCredentialRelayEnvelope(connection, route.CredentialRelayEnvelope{OHTTP: envelope}); err != nil {
		return nil, errors.New("credential relay envelope is unavailable")
	}
	response, err := route.ReadCredentialRelayResponse(connection)
	if err != nil || response.Framing != route.CredentialOHTTPResponse {
		return nil, errors.New("credential relay response is unavailable")
	}
	return append([]byte(nil), response.OHTTP...), nil
}
