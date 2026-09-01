package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// UserPrivateReachabilityRequest is the Endpoint-owned selection for one
// private Target lookup. The Gateway endpoint is deliberately absent: only the
// admitted Initiator obtains that literal URL from its authenticated State
// duty. This request is separate from the Entry attachment later spent on C-2.
type userPrivateReachabilityRequest struct {
	GatewayNodeID, GatewayNodePublicKey [32]byte
	GatewayFamily                       [32]byte
	GatewayProfile                      reachability.GatewayProfile
	StateDigest                         [32]byte
	Epoch                               uint64
	Initiator                           transitPeer
	Entry                               route.EntryAcquirer
	AttachmentID                        [32]byte
	At, Deadline                        time.Time
}

// resolveUserReachability carries one OHTTP request through a newly admitted
// Entry attachment, verifies no descriptor itself, and returns only the
// Gateway's opaque descriptor bytes. The caller must pass those bytes to the
// exact Target verifier before C-2 composition.
func (endpoint *endpoint) resolveUserReachability(ctx context.Context, link string, input userPrivateReachabilityRequest) ([]byte, error) {
	if endpoint == nil || ctx == nil || input.GatewayNodeID == [32]byte{} || input.GatewayNodePublicKey == [32]byte{} || input.GatewayFamily == [32]byte{} ||
		input.GatewayProfile.NodeID != input.GatewayNodeID || input.StateDigest == [32]byte{} || input.Epoch == 0 ||
		!validTransitPeer(input.Initiator) || input.Entry == nil || input.AttachmentID == [32]byte{} || input.At.IsZero() ||
		!input.At.Before(input.Deadline) || input.Deadline.After(input.At.Add(15*time.Second)) {
		return nil, errors.New("user private reachability input is incomplete or outside its bound")
	}
	target, err := endpoint.TargetFromLink(link)
	if err != nil {
		return nil, err
	}
	client, err := reachability.OpenClient(reachability.ClientConfig{NetworkID: endpoint.network, GatewayPublic: input.GatewayNodePublicKey,
		Profile: input.GatewayProfile, At: input.At, Deadline: input.Deadline, Exchange: func(exchangeCtx context.Context, envelope []byte) (reachability.OHTTPResponse, error) {
			return endpoint.exchangePrivateReachability(exchangeCtx, input, envelope)
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

func (endpoint *endpoint) exchangePrivateReachability(ctx context.Context, input userPrivateReachabilityRequest, envelope []byte) (reachability.OHTTPResponse, error) {
	connection, cleanup, err := route.OpenEntryAttachment(ctx, input.Entry, route.EntryAttachmentRequest{NetworkID: endpoint.network,
		Digest: input.StateDigest, Epoch: input.Epoch, AttachmentID: input.AttachmentID, Deadline: input.Deadline})
	if err != nil || connection == nil || cleanup == nil {
		if connection != nil {
			_ = connection.Close()
		}
		return reachability.OHTTPResponse{}, errors.New("user private reachability Entry is unavailable")
	}
	defer cleanup()
	setup := route.ResolutionRelaySetup{NetworkID: endpoint.network, Digest: input.StateDigest, AttachmentID: input.AttachmentID,
		InitiatorNodeID: input.Initiator.NodeID, GatewayNodeID: input.GatewayNodeID, GatewayNodePublicKey: input.GatewayNodePublicKey,
		Epoch: input.Epoch, NotAfter: input.Deadline, EnvelopeCapacity: route.ResolutionEnvelopeCapacity}
	if err := route.WriteResolutionRelaySetup(connection, setup); err != nil {
		return reachability.OHTTPResponse{}, errors.New("user private reachability Initiator setup is unavailable")
	}
	ready, err := route.ReadResolutionRelayReady(connection)
	if err != nil || setup.VerifyResolutionRelayReady(ready) != nil {
		return reachability.OHTTPResponse{}, errors.New("user private reachability Initiator confirmation is invalid")
	}
	if err := route.WriteResolutionRelayEnvelope(connection, route.ResolutionRelayEnvelope{OHTTP: envelope}); err != nil {
		return reachability.OHTTPResponse{}, errors.New("user private reachability envelope is unavailable")
	}
	response, err := route.ReadResolutionRelayResponse(connection)
	if err != nil {
		return reachability.OHTTPResponse{}, errors.New("user private reachability response is unavailable")
	}
	return reachability.OHTTPResponse{Envelope: response.OHTTP, Chunked: response.Framing == route.ResolutionOHTTPChunkedResponse}, nil
}
