package route

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// resolvePrivateReachability sends one OHTTP descriptor lookup through a fresh
// Entry attachment. Gateway selection and profile verification have already
// completed against State; this operation has no direct or alternate carrier.
func (route *Route) resolvePrivateReachability(ctx context.Context, target [32]byte, epoch state.ResolutionEpoch,
	gateway state.DestinationResolutionGateway, profile reachability.GatewayProfile, initiator TransitPeer, attachment [32]byte, at, deadline time.Time,
) ([]byte, error) {
	if route == nil || ctx == nil || target == [32]byte{} || epoch.NetworkID != route.config.NetworkID || epoch.Digest == [32]byte{} || epoch.Number == 0 ||
		gateway.NodeID == [32]byte{} || gateway.PublicKey == [32]byte{} || gateway.Family == [32]byte{} || profile.NodeID != gateway.NodeID ||
		initiator.NodeID == [32]byte{} || initiator.PublicKey == [32]byte{} || attachment == [32]byte{} || at.IsZero() ||
		!at.Before(deadline) || deadline.After(at.Add(userLookupWindow)) {
		return nil, errors.New("user private reachability input is incomplete or outside its bound")
	}
	client, err := reachability.OpenClient(reachability.ClientConfig{NetworkID: route.config.NetworkID, GatewayPublic: gateway.PublicKey,
		Profile: profile, At: at, Deadline: deadline, Exchange: func(exchangeCtx context.Context, envelope []byte) (reachability.OHTTPResponse, error) {
			return route.exchangePrivateReachability(exchangeCtx, epoch, gateway, initiator, attachment, deadline, envelope)
		}})
	if err != nil {
		return nil, errors.Join(errors.New("user private reachability is unavailable"), err)
	}
	descriptor, _, err := client.Resolve(ctx, target)
	if err != nil {
		return nil, errors.Join(errors.New("user private reachability is unavailable"), err)
	}
	return descriptor, nil
}

func (route *Route) exchangePrivateReachability(ctx context.Context, epoch state.ResolutionEpoch, gateway state.DestinationResolutionGateway,
	initiator TransitPeer, attachment [32]byte, deadline time.Time, envelope []byte,
) (reachability.OHTTPResponse, error) {
	connection, cleanup, err := OpenEntryAttachment(ctx, route.config.Entry, EntryAttachmentRequest{NetworkID: route.config.NetworkID,
		Digest: epoch.Digest, Epoch: epoch.Number, AttachmentID: attachment, Deadline: deadline})
	if err != nil || connection == nil || cleanup == nil {
		if connection != nil {
			_ = connection.Close()
		}
		return reachability.OHTTPResponse{}, errors.New("user private reachability Entry is unavailable")
	}
	defer cleanup()
	setup := ResolutionRelaySetup{NetworkID: route.config.NetworkID, Digest: epoch.Digest, AttachmentID: attachment,
		InitiatorNodeID: initiator.NodeID, GatewayNodeID: gateway.NodeID, GatewayNodePublicKey: gateway.PublicKey,
		Epoch: epoch.Number, NotAfter: deadline, EnvelopeCapacity: ResolutionEnvelopeCapacity}
	if err := WriteResolutionRelaySetup(connection, setup); err != nil {
		return reachability.OHTTPResponse{}, errors.New("user private reachability Initiator setup is unavailable")
	}
	ready, err := ReadResolutionRelayReady(connection)
	if err != nil || setup.VerifyResolutionRelayReady(ready) != nil {
		return reachability.OHTTPResponse{}, errors.New("user private reachability Initiator confirmation is invalid")
	}
	if err := WriteResolutionRelayEnvelope(connection, ResolutionRelayEnvelope{OHTTP: envelope}); err != nil {
		return reachability.OHTTPResponse{}, errors.New("user private reachability envelope is unavailable")
	}
	response, err := ReadResolutionRelayResponse(connection)
	if err != nil {
		return reachability.OHTTPResponse{}, errors.New("user private reachability response is unavailable")
	}
	return reachability.OHTTPResponse{Envelope: response.OHTTP, Chunked: response.Framing == ResolutionOHTTPChunkedResponse}, nil
}

// exchangeCredential is Route's exact carrier for Endpoint's durable Grant
// adapter. It opens a distinct Entry attachment and names only State-selected
// Initiator and issuer identities in the closed relay setup.
func (route *Route) exchangeCredential(ctx context.Context, epoch state.ResolutionEpoch, initiator TransitPeer, issuer state.TransitIssuer,
	attachment [32]byte, deadline time.Time, envelope []byte,
) ([]byte, error) {
	if route == nil || ctx == nil || epoch.NetworkID != route.config.NetworkID || epoch.Digest == [32]byte{} || epoch.Number == 0 ||
		initiator.NodeID == [32]byte{} || issuer.NodeID == [32]byte{} || issuer.PublicKey == [32]byte{} || attachment == [32]byte{} ||
		deadline.IsZero() || len(envelope) == 0 || len(envelope) > CredentialEnvelopeCapacity {
		return nil, errors.New("credential relay exchange is invalid")
	}
	connection, cleanup, err := OpenEntryAttachment(ctx, route.config.Entry, EntryAttachmentRequest{NetworkID: route.config.NetworkID,
		Digest: epoch.Digest, Epoch: epoch.Number, AttachmentID: attachment, Deadline: deadline})
	if err != nil || connection == nil || cleanup == nil {
		if connection != nil {
			_ = connection.Close()
		}
		return nil, errors.New("credential relay Entry is unavailable")
	}
	defer cleanup()
	setup := CredentialRelaySetup{NetworkID: route.config.NetworkID, Digest: epoch.Digest, Epoch: epoch.Number, AttachmentID: attachment,
		InitiatorNodeID: initiator.NodeID, IssuerNodeID: issuer.NodeID, IssuerNodePublicKey: issuer.PublicKey,
		IssuerProfileDigest: sha256.Sum256(issuer.Profile), NotAfter: deadline, EnvelopeCapacity: CredentialEnvelopeCapacity}
	if err := WriteCredentialRelaySetup(connection, setup); err != nil {
		return nil, errors.New("credential relay Initiator setup is unavailable")
	}
	ready, err := ReadCredentialRelayReady(connection)
	if err != nil || setup.VerifyCredentialRelayReady(ready) != nil {
		return nil, errors.New("credential relay Initiator confirmation is invalid")
	}
	if err := WriteCredentialRelayEnvelope(connection, CredentialRelayEnvelope{OHTTP: envelope}); err != nil {
		return nil, errors.New("credential relay envelope is unavailable")
	}
	response, err := ReadCredentialRelayResponse(connection)
	if err != nil || response.Framing != CredentialOHTTPResponse {
		return nil, errors.New("credential relay response is unavailable")
	}
	return append([]byte(nil), response.OHTTP...), nil
}
