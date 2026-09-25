package endpoint

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// transitCredentialIssuerView is the exact issuer projection shared by the
// Application and Publisher State views. Descriptor v1 continues its
// fixed-Grant path, while Descriptor v2 fails closed when State cannot project
// this issuer duty.
type transitCredentialIssuerView interface {
	CredentialIssuer(time.Time, time.Time) (state.TransitIssuer, bool)
}

type transitCredentialSubmission struct {
	authorization []byte
	attachment    [32]byte
	certificate   tls.Certificate
	finish        func(bool) error
}

// applicationServiceAttachment verifies the fixed Grant carried by a retained
// Descriptor v1 publisher path. Decode failure is rejected before an
// attachment identifier can be allocated.
func applicationServiceAttachment(authorization []byte, epoch state.ResolutionEpoch, introduction [32]byte, notAfter time.Time) ([32]byte, error) {
	grant, err := route.DecodeTransitGrant(authorization)
	if err != nil {
		return [32]byte{}, errors.New("application Transit Grant is malformed")
	}
	var authority ed25519.PublicKey
	for _, candidate := range epoch.Authorities {
		if candidate.ID == grant.IssuerID {
			authority = ed25519.PublicKey(candidate.PublicKey[:])
			break
		}
	}
	if authority == nil {
		return [32]byte{}, errors.New("introduction transit grant issuer is absent from current state")
	}
	grant, err = route.VerifyTransitGrant(authorization, authority)
	if err != nil || grant.NetworkID != epoch.NetworkID || grant.Digest != epoch.Digest || grant.Epoch != epoch.Number ||
		grant.TransitRole != route.IntroductionRole || grant.TransitNodeID != introduction || grant.AttachmentID == [32]byte{} ||
		notAfter.IsZero() || notAfter.After(grant.NotAfter) {
		return [32]byte{}, errors.New("introduction transit grant does not bind the current Application route")
	}
	return grant.AttachmentID, nil
}

func applicationAttachmentID() ([32]byte, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil || value == [32]byte{} {
		return [32]byte{}, errors.New("application Connection could not create a Route attachment identifier")
	}
	return value, nil
}

func (endpoint *endpoint) acquireTransitCredential(ctx context.Context, view transitCredentialIssuerView, epoch state.ResolutionEpoch,
	entry applicationEntry, initiator, transit transitPeer, role byte, slot reachability.Introduction,
	at, deadline time.Time,
) (transitCredentialSubmission, error) {
	if entry == nil {
		return transitCredentialSubmission{}, errors.New("credential relay Entry owner is unavailable")
	}
	if slot.SubmissionMode == reachability.SubmissionFixedGrant {
		if role != route.IntroductionRole {
			return transitCredentialSubmission{}, errors.New("fixed reachability Grant is only valid for Introduction")
		}
		attachment, err := applicationServiceAttachment(slot.SubmissionAuthorization, epoch, transit.NodeID, slot.NotAfter)
		if err != nil {
			return transitCredentialSubmission{}, err
		}
		// Descriptor v1 already carries its fixed Grant; the Publisher path keeps
		// this exact capability rather than acquiring a membership credential.
		return transitCredentialSubmission{attachment: attachment}, nil
	}
	if slot.SubmissionMode != reachability.SubmissionMembershipGrant {
		return transitCredentialSubmission{}, errors.New("reachability descriptor submission mode is unsupported")
	}
	if view == nil {
		return transitCredentialSubmission{}, errors.New("current State does not project a transit issuer")
	}
	issuer, available := view.CredentialIssuer(at, deadline)
	if !available || issuer.NodeID == initiator.NodeID || issuer.NodeID == transit.NodeID || issuer.Family == initiator.Family || issuer.Family == transit.Family {
		return transitCredentialSubmission{}, errors.New("current State transit issuer is unavailable or overlaps the route")
	}
	profile, err := credential.DecodeProfile(issuer.Profile)
	if err != nil || credential.VerifyProfile(profile, endpoint.network, issuer.NodeID, issuer.PublicKey, at, deadline) != nil {
		return transitCredentialSubmission{}, errors.New("current State transit issuer profile is invalid")
	}
	owner, err := endpoint.transitAcquire.owner(role)
	if err != nil {
		return transitCredentialSubmission{}, errors.New("endpoint transit acquisition owner is unavailable")
	}
	carrierAttachment, err := applicationAttachmentID()
	if err != nil {
		return transitCredentialSubmission{}, errors.New("credential relay attachment is unavailable")
	}
	scope := transitAcquisitionScope{NetworkID: endpoint.network, Digest: epoch.Digest, Epoch: epoch.Number,
		IssuerNodeID: issuer.NodeID, IssuerPublicKey: issuer.PublicKey, IssuerProfileDigest: sha256.Sum256(issuer.Profile),
		GrantSignerPublicKey: profile.GrantSignerPublicKey, TransitNodeID: transit.NodeID, TransitRole: role,
		NotAfter: deadline}
	acquired, err := owner.acquire(ctx, scope, func(issueCtx context.Context, request credential.Request) (credential.Result, error) {
		client, err := credential.OpenClient(credential.ClientConfig{NetworkID: endpoint.network, IssuerPublic: issuer.PublicKey, Profile: profile,
			At: at, Deadline: deadline, Exchange: func(exchangeCtx context.Context, envelope []byte) ([]byte, error) {
				return endpoint.exchangeTransitCredential(exchangeCtx, entry, epoch, initiator, issuer, carrierAttachment, deadline, envelope)
			}})
		if err != nil {
			return credential.Result{}, err
		}
		return client.Issue(issueCtx, request)
	}, endpoint.enrollTransitClient)
	if err != nil {
		return transitCredentialSubmission{}, err
	}
	return transitCredentialSubmission{authorization: acquired.attempt.Grant, attachment: acquired.attempt.Request.AttachmentID,
		certificate: acquired.attempt.Certificate, finish: acquired.finish}, nil
}

func (endpoint *endpoint) exchangeTransitCredential(ctx context.Context, source route.EntryAcquirer, epoch state.ResolutionEpoch,
	initiator transitPeer, issuer state.TransitIssuer, attachment [32]byte, deadline time.Time, envelope []byte) ([]byte, error) {
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
