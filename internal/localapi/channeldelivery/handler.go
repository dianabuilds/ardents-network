package channeldelivery

import (
	"context"
	"errors"
	"time"

	authoritydomain "ardents/internal/authority"
	domain "ardents/internal/channeldelivery"
	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"
	"ardents/internal/storage"

	"connectrpc.com/connect"
)

type Service interface {
	Prepare(context.Context, domain.Command, domain.PrepareRequest) (identityapi.CapabilityDeliveryAttestation, error)
	Install(context.Context, domain.Command, uint32, identitycapability.SealedGenerationDelivery) (identitycapability.GenerationDeliveryReceipt, error)
	Activate(context.Context, domain.Command, uint32, identitycapability.GenerationActivation) (identitycapability.GenerationDeliveryReceipt, error)
	AdoptAuthorityTransition(domain.Command, uint32, string, authoritydomain.AuthorityTransition) error
	FinalizeAuthorityTransition(domain.Command, uint32, string, authoritydomain.AuthorityTransitionRecord) error
}

const maximumAuthorityTransitionArtifactBytes = 64 << 10

func (h *ChannelDeliveryEndpoint) AdoptAuthorityTransition(
	ctx context.Context,
	request *connect.Request[protocol.AdoptAuthorityTransitionRequest],
) (*connect.Response[protocol.AuthorityTrustTransitionResponse], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*protocol.AuthorityTrustTransitionResponse, *rpc.Error) {
		command, ok := commandFromCall(call)
		raw := request.Msg.GetTransitionJson()
		if h.service == nil || !ok || len(raw) == 0 ||
			len(raw) > maximumAuthorityTransitionArtifactBytes {
			return nil, deliveryError("adopt_authority_transition", domain.ErrInvalidArgument)
		}
		var transition authoritydomain.AuthorityTransition
		if storage.DecodeJSONStrict(raw, &transition) != nil {
			return nil, deliveryError("adopt_authority_transition", domain.ErrInvalidArgument)
		}
		if err := h.service.AdoptAuthorityTransition(
			command, request.Msg.GetVersion(), request.Msg.GetRealmId(), transition,
		); err != nil {
			return nil, deliveryError("adopt_authority_transition", err)
		}
		return &protocol.AuthorityTrustTransitionResponse{
			Status: &protocol.OperationStatus{State: "adopted", Accepted: true},
		}, nil
	})
}

func (h *ChannelDeliveryEndpoint) FinalizeAuthorityTransition(
	ctx context.Context,
	request *connect.Request[protocol.FinalizeAuthorityTransitionRequest],
) (*connect.Response[protocol.AuthorityTrustTransitionResponse], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*protocol.AuthorityTrustTransitionResponse, *rpc.Error) {
		command, ok := commandFromCall(call)
		raw := request.Msg.GetTransitionRecordJson()
		if h.service == nil || !ok || len(raw) == 0 ||
			len(raw) > maximumAuthorityTransitionArtifactBytes {
			return nil, deliveryError("finalize_authority_transition", domain.ErrInvalidArgument)
		}
		var record authoritydomain.AuthorityTransitionRecord
		if storage.DecodeJSONStrict(raw, &record) != nil {
			return nil, deliveryError("finalize_authority_transition", domain.ErrInvalidArgument)
		}
		if err := h.service.FinalizeAuthorityTransition(
			command, request.Msg.GetVersion(), request.Msg.GetRealmId(), record,
		); err != nil {
			return nil, deliveryError("finalize_authority_transition", err)
		}
		return &protocol.AuthorityTrustTransitionResponse{
			Status: &protocol.OperationStatus{State: "finalized", Accepted: true},
		}, nil
	})
}

func (h *ChannelDeliveryEndpoint) ActivateGeneration(ctx context.Context, request *connect.Request[protocol.ActivateGenerationRequest]) (*connect.Response[protocol.ActivateGenerationResponse], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*protocol.ActivateGenerationResponse, *rpc.Error) {
		command, ok := commandFromCall(call)
		if h.service == nil || !ok {
			return nil, deliveryError("activate", domain.ErrUnavailable)
		}
		activation, err := activationFromWire(request.Msg.GetActivation())
		if err != nil {
			return nil, deliveryError("activate", domain.ErrInvalidArgument)
		}
		mutationContext, cancel := rpc.MutationContext(ctx)
		defer cancel()
		receipt, err := h.service.Activate(
			mutationContext, command, request.Msg.GetVersion(), activation,
		)
		if err != nil {
			return nil, deliveryError("activate", err)
		}
		return &protocol.ActivateGenerationResponse{
			Status:  &protocol.OperationStatus{State: receipt.Phase, Accepted: true},
			Receipt: receiptToWire(receipt),
		}, nil
	})
}

type ChannelDeliveryEndpoint struct{ service Service }

func NewHandler(service Service) *ChannelDeliveryEndpoint {
	return &ChannelDeliveryEndpoint{service: service}
}

func (h *ChannelDeliveryEndpoint) PrepareGenerationDelivery(ctx context.Context, request *connect.Request[protocol.PrepareGenerationDeliveryRequest]) (*connect.Response[protocol.PrepareGenerationDeliveryResponse], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*protocol.PrepareGenerationDeliveryResponse, *rpc.Error) {
		command, ok := commandFromCall(call)
		if h.service == nil || !ok {
			return nil, deliveryError("prepare", domain.ErrUnavailable)
		}
		if request.Msg.GetValidForSeconds() > uint64(domain.MaximumAttestationValidity/time.Second) {
			return nil, deliveryError("prepare", domain.ErrInvalidArgument)
		}
		mutationContext, cancel := rpc.MutationContext(ctx)
		defer cancel()
		attestation, err := h.service.Prepare(mutationContext, command, domain.PrepareRequest{
			Version: request.Msg.GetVersion(), SubjectPrincipal: request.Msg.GetSubjectPrincipal(),
			ValidFor: time.Duration(request.Msg.GetValidForSeconds()) * time.Second,
		})
		if err != nil {
			return nil, deliveryError("prepare", err)
		}
		return &protocol.PrepareGenerationDeliveryResponse{
			Status:      &protocol.OperationStatus{State: "prepared", Accepted: true},
			Attestation: attestationToWire(attestation),
		}, nil
	})
}

func (h *ChannelDeliveryEndpoint) InstallGenerationDelivery(ctx context.Context, request *connect.Request[protocol.InstallGenerationDeliveryRequest]) (*connect.Response[protocol.InstallGenerationDeliveryResponse], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*protocol.InstallGenerationDeliveryResponse, *rpc.Error) {
		command, ok := commandFromCall(call)
		if h.service == nil || !ok {
			return nil, deliveryError("install", domain.ErrUnavailable)
		}
		sealed, err := sealedFromWire(request.Msg.GetSealed())
		if err != nil {
			return nil, deliveryError("install", domain.ErrInvalidArgument)
		}
		mutationContext, cancel := rpc.MutationContext(ctx)
		defer cancel()
		receipt, err := h.service.Install(mutationContext, command, request.Msg.GetVersion(), sealed)
		if err != nil {
			return nil, deliveryError("install", err)
		}
		return &protocol.InstallGenerationDeliveryResponse{
			Status:  &protocol.OperationStatus{State: receipt.Phase, Accepted: true},
			Receipt: receiptToWire(receipt),
		}, nil
	})
}

func commandFromCall(call rpc.Call) (domain.Command, bool) {
	authorized, ok := call.Authorized()
	if !ok {
		return domain.Command{}, false
	}
	return domain.Command{Actor: authorized.Actor(), Effective: authorized.Effective()}, true
}

func deliveryError(operation string, err error) *rpc.Error {
	code, category, retryable := "channel_delivery_internal", "internal_failure", false
	switch {
	case errors.Is(err, domain.ErrUnsupportedVersion):
		code, category = "channel_delivery_unsupported_version", "invalid_input"
	case errors.Is(err, domain.ErrInvalidArgument):
		code, category = "channel_delivery_invalid_argument", "invalid_input"
	case errors.Is(err, domain.ErrPermissionDenied):
		code, category = "channel_delivery_forbidden", "forbidden"
	case errors.Is(err, domain.ErrUnavailable), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		code, category, retryable = "channel_delivery_unavailable", "unavailable", true
	}
	return &rpc.Error{
		Code: code, Category: category, Message: "Channel delivery request failed",
		Domain: "authority", Operation: operation, Reason: code, Retryable: retryable,
		Details: map[string]any{},
	}
}
