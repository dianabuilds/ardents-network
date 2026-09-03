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

// acquireUserRouteCredential is Endpoint's adapter for Route's already
// selected membership Grant tuple. Route owns the Entry/Initiator carrier and
// the exact User-route peers; Endpoint owns only the durable at-most-once
// request, local key enrollment, and terminal spend record.
func (endpoint *endpoint) acquireUserRouteCredential(ctx context.Context, input route.CredentialRequest) (route.Credential, error) {
	if endpoint == nil || ctx == nil || input.Epoch.NetworkID != endpoint.network || input.Epoch.Digest == [32]byte{} || input.Epoch.Number == 0 ||
		input.TransitRole != route.IntroductionRole || input.AttachmentID == [32]byte{} || input.At.IsZero() || !input.At.Before(input.NotAfter) || input.Exchange == nil ||
		!validRouteTransitPeer(input.Initiator) || !validRouteTransitPeer(input.Transit) || input.Issuer.NodeID == [32]byte{} ||
		input.Issuer.PublicKey == [32]byte{} || input.Issuer.Family == [32]byte{} {
		return route.Credential{}, errors.New("user Route credential request is incomplete")
	}
	profile, err := credential.DecodeProfile(input.Issuer.Profile)
	if err != nil || credential.VerifyProfile(profile, endpoint.network, input.Issuer.NodeID, input.Issuer.PublicKey, input.At, input.NotAfter) != nil {
		return route.Credential{}, errors.New("current State transit issuer profile is invalid")
	}
	owner, err := endpoint.transitAcquire.owner(input.TransitRole)
	if err != nil {
		return route.Credential{}, errors.New("endpoint transit acquisition owner is unavailable")
	}
	scope := transitAcquisitionScope{NetworkID: endpoint.network, Digest: input.Epoch.Digest, Epoch: input.Epoch.Number,
		IssuerNodeID: input.Issuer.NodeID, IssuerPublicKey: input.Issuer.PublicKey, IssuerProfileDigest: sha256.Sum256(input.Issuer.Profile),
		GrantSignerPublicKey: profile.GrantSignerPublicKey, TransitNodeID: input.Transit.NodeID, AttachmentID: input.AttachmentID,
		TransitRole: input.TransitRole, NotAfter: input.NotAfter}
	attempt, err := owner.begin(scope)
	if err != nil {
		return route.Credential{}, err
	}
	if attempt.Phase == transitPending {
		client, err := credential.OpenClient(credential.ClientConfig{NetworkID: endpoint.network, IssuerPublic: input.Issuer.PublicKey, Profile: profile,
			At: input.At, Deadline: input.NotAfter, Exchange: credential.Exchange(input.Exchange)})
		if err != nil {
			_ = owner.fail()
			return route.Credential{}, transitAcquisitionOutcomeError{outcome: credential.Unavailable}
		}
		result, err := client.Issue(ctx, attempt.Request)
		if err != nil {
			if !errors.Is(err, credential.ErrExchangeUnavailable) {
				_ = owner.fail()
			}
			return route.Credential{}, transitAcquisitionOutcomeError{outcome: credential.Unavailable}
		}
		if err := owner.commit(result); err != nil {
			return route.Credential{}, err
		}
		if result.Outcome != credential.Issued {
			return route.Credential{}, transitAcquisitionOutcomeError{outcome: result.Outcome}
		}
		attempt, err = owner.begin(scope)
		if err != nil {
			return route.Credential{}, err
		}
	}
	presenting, err := owner.present(scope)
	if err != nil {
		return route.Credential{}, err
	}
	erase, err := endpoint.enrollTransitClient(presenting.Grant, presenting.Certificate)
	if err != nil {
		_ = owner.finish(false)
		return route.Credential{}, err
	}
	return route.Credential{Authorization: append([]byte(nil), presenting.Grant...), ClientCertificate: presenting.Certificate,
		Finish: func(presented bool) error {
			erase()
			return owner.finish(presented)
		}}, nil
}

func validRouteTransitPeer(peer route.TransitPeer) bool {
	return peer.NodeID != [32]byte{} && peer.PublicKey != [32]byte{} && peer.Family != [32]byte{} && peer.Endpoint != ""
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
	attempt, err := owner.begin(scope)
	if err != nil {
		return transitCredentialSubmission{}, err
	}
	if attempt.Phase == transitPending {
		client, err := credential.OpenClient(credential.ClientConfig{NetworkID: endpoint.network, IssuerPublic: issuer.PublicKey, Profile: profile,
			At: at, Deadline: deadline, Exchange: func(exchangeCtx context.Context, envelope []byte) ([]byte, error) {
				return endpoint.exchangeTransitCredential(exchangeCtx, entry, epoch, initiator, issuer, carrierAttachment, deadline, envelope)
			}})
		if err != nil {
			_ = owner.fail()
			return transitCredentialSubmission{}, transitAcquisitionOutcomeError{outcome: credential.Unavailable}
		}
		result, err := client.Issue(ctx, attempt.Request)
		if err != nil {
			if !errors.Is(err, credential.ErrExchangeUnavailable) {
				_ = owner.fail()
			}
			return transitCredentialSubmission{}, transitAcquisitionOutcomeError{outcome: credential.Unavailable}
		}
		if err := owner.commit(result); err != nil {
			return transitCredentialSubmission{}, err
		}
		if result.Outcome != credential.Issued {
			return transitCredentialSubmission{}, transitAcquisitionOutcomeError{outcome: result.Outcome}
		}
		attempt, err = owner.begin(scope)
		if err != nil {
			return transitCredentialSubmission{}, err
		}
	}
	presenting, err := owner.present(scope)
	if err != nil {
		return transitCredentialSubmission{}, err
	}
	erase, err := endpoint.enrollTransitClient(presenting.Grant, presenting.Certificate)
	if err != nil {
		_ = owner.finish(false)
		return transitCredentialSubmission{}, err
	}
	return transitCredentialSubmission{authorization: presenting.Grant, attachment: presenting.Request.AttachmentID,
		certificate: presenting.Certificate, finish: func(presented bool) error {
			erase()
			return owner.finish(presented)
		}}, nil
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
