package authority

import (
	"context"
	"errors"
	"time"

	domain "ardents/internal/authority"
	identityapi "ardents/internal/identity"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

type Service interface {
	CreateOrReopen(context.Context, domain.Command, domain.CreateRequest) (domain.CreateResult, error)
	Inspect(context.Context, domain.Command, domain.InspectRequest) (domain.Status, error)
	IssueInitialGeneration(context.Context, domain.Command, domain.InitialGenerationRequest) (domain.InitialGenerationResult, error)
	AcknowledgeInitialGeneration(context.Context, domain.Command, domain.InitialGenerationAcknowledgeRequest) (domain.InitialGenerationAcknowledgeResult, error)
	RotateChannel(context.Context, domain.Command, domain.RotationRequest) (domain.RotationResult, error)
	CommitChannelActivation(context.Context, domain.Command, domain.ActivationCommitRequest) (domain.ActivationCommitResult, error)
	AcknowledgeChannelActivation(context.Context, domain.Command, domain.ActivationAcknowledgeRequest) (domain.ActivationAcknowledgeResult, error)
	Readiness() domain.Status
}

func (h *AuthorityEndpoint) RotateChannel(ctx context.Context, request *connect.Request[protocol.RotateChannelRequest]) (*connect.Response[protocol.RotateChannelResponse], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*protocol.RotateChannelResponse, *rpc.Error) {
		if h.service == nil {
			return nil, authorityError("rotate_channel", domain.ErrUnavailable)
		}
		command, ok := authorityCommand(call)
		if !ok {
			return nil, authorityError("rotate_channel", domain.ErrPermissionDenied)
		}
		channelID, err := fixedID(request.Msg.GetChannelId())
		if err != nil || len(request.Msg.GetRecipientAttestations()) == 0 ||
			len(request.Msg.GetRecipientAttestations()) > domain.MaxMembersPerChannel ||
			request.Msg.GetValidForSeconds() > uint64((30*24*time.Hour)/time.Second) ||
			request.Msg.GetDrainForSeconds() > uint64(domain.MaximumPreviousGenerationDrain/time.Second) {
			return nil, authorityError("rotate_channel", domain.ErrInvalidArgument)
		}
		attestations := make([]identityapi.CapabilityDeliveryAttestation, 0, len(request.Msg.GetRecipientAttestations()))
		for _, wire := range request.Msg.GetRecipientAttestations() {
			attestation, mapErr := attestationFromWire(wire)
			if mapErr != nil {
				return nil, authorityError("rotate_channel", domain.ErrInvalidArgument)
			}
			attestations = append(attestations, attestation)
		}
		mutationContext, cancel := rpc.MutationContext(ctx)
		defer cancel()
		result, err := h.service.RotateChannel(mutationContext, command, domain.RotationRequest{
			Version: request.Msg.GetVersion(), RequestID: request.Msg.GetRequestId(),
			RealmID: request.Msg.GetRealmId(), ChannelID: channelID,
			RecipientAttestations: attestations,
			ValidFor:              time.Duration(request.Msg.GetValidForSeconds()) * time.Second,
			DrainFor:              time.Duration(request.Msg.GetDrainForSeconds()) * time.Second,
		})
		if err != nil {
			return nil, authorityError("rotate_channel", err)
		}
		deliveries := make([]*protocol.RotationDelivery, 0, len(result.Deliveries))
		for _, delivery := range result.Deliveries {
			deliveries = append(deliveries, &protocol.RotationDelivery{
				DeliveryId:         delivery.DeliveryID,
				RecipientPrincipal: delivery.RecipientPrincipal,
				Sealed:             sealedToWire(delivery.Sealed),
			})
		}
		return &protocol.RotateChannelResponse{
			Status:  &protocol.OperationStatus{State: result.Phase, Accepted: true},
			RealmId: result.RealmID, OperationId: result.OperationID,
			AuthoritySequence: result.AuthoritySequence, ChannelId: result.ChannelID[:],
			PreviousGeneration: result.PreviousGeneration,
			PendingGeneration:  result.PendingGeneration, Phase: result.Phase,
			Deliveries: deliveries,
		}, nil
	})
}

func (h *AuthorityEndpoint) CommitChannelActivation(ctx context.Context, request *connect.Request[protocol.CommitChannelActivationRequest]) (*connect.Response[protocol.CommitChannelActivationResponse], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*protocol.CommitChannelActivationResponse, *rpc.Error) {
		if h.service == nil {
			return nil, authorityError("commit_channel_activation", domain.ErrUnavailable)
		}
		command, ok := authorityCommand(call)
		if !ok {
			return nil, authorityError("commit_channel_activation", domain.ErrPermissionDenied)
		}
		mutationContext, cancel := rpc.MutationContext(ctx)
		defer cancel()
		result, err := h.service.CommitChannelActivation(
			mutationContext, command, domain.ActivationCommitRequest{
				Version: request.Msg.GetVersion(), RealmID: request.Msg.GetRealmId(),
				OperationID: request.Msg.GetOperationId(),
			},
		)
		if err != nil {
			return nil, authorityError("commit_channel_activation", err)
		}
		return &protocol.CommitChannelActivationResponse{
			Status:  &protocol.OperationStatus{State: result.Phase, Accepted: true},
			RealmId: result.RealmID, OperationId: result.OperationID,
			AuthoritySequence: result.AuthoritySequence, Phase: result.Phase,
			Activation: activationToWire(result.Activation),
		}, nil
	})
}

func (h *AuthorityEndpoint) AcknowledgeChannelActivation(ctx context.Context, request *connect.Request[protocol.AcknowledgeChannelActivationRequest]) (*connect.Response[protocol.AcknowledgeChannelActivationResponse], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*protocol.AcknowledgeChannelActivationResponse, *rpc.Error) {
		if h.service == nil {
			return nil, authorityError("acknowledge_channel_activation", domain.ErrUnavailable)
		}
		command, ok := authorityCommand(call)
		if !ok {
			return nil, authorityError("acknowledge_channel_activation", domain.ErrPermissionDenied)
		}
		receipt, err := receiptFromWire(request.Msg.GetReceipt())
		if err != nil {
			return nil, authorityError("acknowledge_channel_activation", domain.ErrInvalidArgument)
		}
		mutationContext, cancel := rpc.MutationContext(ctx)
		defer cancel()
		result, err := h.service.AcknowledgeChannelActivation(
			mutationContext, command, domain.ActivationAcknowledgeRequest{
				Version: request.Msg.GetVersion(), RealmID: request.Msg.GetRealmId(),
				OperationID:  request.Msg.GetOperationId(),
				ApprovedHost: request.Msg.GetApprovedHost(), Receipt: receipt,
			},
		)
		if err != nil {
			return nil, authorityError("acknowledge_channel_activation", err)
		}
		return &protocol.AcknowledgeChannelActivationResponse{
			Status:  &protocol.OperationStatus{State: result.Phase, Accepted: true},
			RealmId: result.RealmID, OperationId: result.OperationID,
			AuthoritySequence: result.AuthoritySequence, Phase: result.Phase,
			CurrentGeneration:  result.CurrentGeneration,
			PreviousGeneration: result.PreviousGeneration,
			DrainDeadline:      rpc.Timestamp(result.DrainDeadline),
		}, nil
	})
}

func (h *AuthorityEndpoint) IssueInitialGeneration(ctx context.Context, request *connect.Request[protocol.IssueInitialGenerationRequest]) (*connect.Response[protocol.IssueInitialGenerationResponse], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*protocol.IssueInitialGenerationResponse, *rpc.Error) {
		if h.service == nil {
			return nil, authorityError("issue_initial_generation", domain.ErrUnavailable)
		}
		command, ok := authorityCommand(call)
		if !ok {
			return nil, authorityError("issue_initial_generation", domain.ErrPermissionDenied)
		}
		attestation, err := attestationFromWire(request.Msg.GetRecipientAttestation())
		if err != nil || request.Msg.GetValidForSeconds() > uint64((30*24*time.Hour)/time.Second) {
			return nil, authorityError("issue_initial_generation", domain.ErrInvalidArgument)
		}
		mutationContext, cancel := rpc.MutationContext(ctx)
		defer cancel()
		result, err := h.service.IssueInitialGeneration(mutationContext, command, domain.InitialGenerationRequest{
			Version: request.Msg.GetVersion(), RequestID: request.Msg.GetRequestId(),
			RealmID:              request.Msg.GetRealmId(),
			ChannelClass:         identityapi.CapabilityScope(request.Msg.GetChannelClass()),
			Permissions:          identityapi.CapabilityPermission(request.Msg.GetPermissions()),
			RecipientAttestation: attestation,
			ValidFor:             time.Duration(request.Msg.GetValidForSeconds()) * time.Second,
		})
		if err != nil {
			return nil, authorityError("issue_initial_generation", err)
		}
		return &protocol.IssueInitialGenerationResponse{
			Status:  &protocol.OperationStatus{State: domain.DeliveryPhaseIssued, Accepted: true},
			RealmId: result.RealmID, OperationId: result.OperationID, DeliveryId: result.DeliveryID,
			AuthoritySequence: result.AuthoritySequence, ChannelId: result.ChannelID[:],
			Generation: result.Generation, Sealed: sealedToWire(result.Sealed),
		}, nil
	})
}

func (h *AuthorityEndpoint) AcknowledgeInitialGeneration(ctx context.Context, request *connect.Request[protocol.AcknowledgeInitialGenerationRequest]) (*connect.Response[protocol.AcknowledgeInitialGenerationResponse], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*protocol.AcknowledgeInitialGenerationResponse, *rpc.Error) {
		if h.service == nil {
			return nil, authorityError("acknowledge_initial_generation", domain.ErrUnavailable)
		}
		command, ok := authorityCommand(call)
		if !ok {
			return nil, authorityError("acknowledge_initial_generation", domain.ErrPermissionDenied)
		}
		receipt, err := receiptFromWire(request.Msg.GetReceipt())
		if err != nil {
			return nil, authorityError("acknowledge_initial_generation", domain.ErrInvalidArgument)
		}
		mutationContext, cancel := rpc.MutationContext(ctx)
		defer cancel()
		result, err := h.service.AcknowledgeInitialGeneration(
			mutationContext, command, domain.InitialGenerationAcknowledgeRequest{
				Version: request.Msg.GetVersion(), RealmID: request.Msg.GetRealmId(), Receipt: receipt,
			},
		)
		if err != nil {
			return nil, authorityError("acknowledge_initial_generation", err)
		}
		return &protocol.AcknowledgeInitialGenerationResponse{
			Status:  &protocol.OperationStatus{State: result.Phase, Accepted: true},
			RealmId: result.RealmID, DeliveryId: result.DeliveryID,
			AuthoritySequence: result.AuthoritySequence, Phase: result.Phase,
		}, nil
	})
}

type AuthorityEndpoint struct{ service Service }

func NewHandler(service Service) (*AuthorityEndpoint, error) {
	return &AuthorityEndpoint{service: service}, nil
}

func (h *AuthorityEndpoint) CreateRealmAuthority(ctx context.Context, request *connect.Request[protocol.CreateRealmAuthorityRequest]) (*connect.Response[protocol.CreateRealmAuthorityResponse], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*protocol.CreateRealmAuthorityResponse, *rpc.Error) {
		if h.service == nil {
			return nil, authorityError("create", domain.ErrUnavailable)
		}
		command, ok := authorityCommand(call)
		if !ok {
			return nil, authorityError("create", domain.ErrPermissionDenied)
		}
		mutationContext, cancel := rpc.MutationContext(ctx)
		defer cancel()
		result, err := h.service.CreateOrReopen(mutationContext, command, domain.CreateRequest{
			Version: request.Msg.GetVersion(), RequestID: request.Msg.GetRequestId(),
			RealmClass: request.Msg.GetRealmClass(),
		})
		if err != nil {
			return nil, authorityError("create", err)
		}
		status := h.service.Readiness()
		return &protocol.CreateRealmAuthorityResponse{
			Status:      operationStatus(status),
			Authority:   mapStatus(status),
			OperationId: result.OperationID,
		}, nil
	})
}

func (h *AuthorityEndpoint) InspectRealmAuthority(ctx context.Context, request *connect.Request[protocol.InspectRealmAuthorityRequest]) (*connect.Response[protocol.InspectRealmAuthorityResponse], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*protocol.InspectRealmAuthorityResponse, *rpc.Error) {
		if h.service == nil {
			return nil, authorityError("inspect", domain.ErrUnavailable)
		}
		command, ok := authorityCommand(call)
		if !ok {
			return nil, authorityError("inspect", domain.ErrPermissionDenied)
		}
		status, err := h.service.Inspect(ctx, command, domain.InspectRequest{
			Version: request.Msg.GetVersion(), RealmID: request.Msg.GetRealmId(),
		})
		if err != nil {
			return nil, authorityError("inspect", err)
		}
		return &protocol.InspectRealmAuthorityResponse{
			Status: operationStatus(status), Authority: mapStatus(status),
		}, nil
	})
}

func authorityCommand(call rpc.Call) (domain.Command, bool) {
	authorized, ok := call.Authorized()
	if !ok {
		return domain.Command{}, false
	}
	resource := authorized.Resource()
	return domain.Command{
		Actor: authorized.Actor(), Effective: authorized.Effective(),
		Action: string(authorized.Action()), ResourceKind: string(resource.Kind), ResourceID: resource.ID,
	}, true
}

func operationStatus(status domain.Status) *protocol.OperationStatus {
	return &protocol.OperationStatus{
		State: status.Phase, Reason: status.Reason,
		Accepted: status.Phase == domain.PhaseReady,
	}
}

func mapStatus(status domain.Status) *protocol.AuthorityStatusSnapshot {
	return &protocol.AuthorityStatusSnapshot{
		Version: status.Version, SchemaVersion: status.SchemaVersion,
		RealmId: status.RealmID, RealmClass: status.RealmClass,
		AuthorityEpoch: status.AuthorityEpoch, AuthoritySequence: status.AuthoritySequence,
		CheckpointDigest: status.CheckpointDigest, Phase: status.Phase,
		Readiness: status.Readiness, Reason: status.Reason,
		MemberCount: status.MemberCount, ChannelCount: status.ChannelCount,
		PendingOperationCount: status.PendingOperationCount,
		AuditOutboxDepth:      status.AuditOutboxDepth,
		CurrentGeneration:     status.CurrentGeneration,
		OperationDeadline:     rpc.Timestamp(status.OperationDeadline),
	}
}

func authorityError(operation string, err error) *rpc.Error {
	code, category, retryable := "authority_internal", "internal_failure", false
	switch {
	case errors.Is(err, domain.ErrUnsupportedVersion):
		code, category = "authority_unsupported_version", "invalid_input"
	case errors.Is(err, domain.ErrInvalidArgument):
		code, category = "authority_invalid_argument", "invalid_input"
	case errors.Is(err, domain.ErrPermissionDenied):
		code, category = "authority_forbidden", "forbidden"
	case errors.Is(err, domain.ErrConflict):
		code, category = "authority_conflict", "conflict"
	case errors.Is(err, domain.ErrResourceExhausted):
		code, category = "authority_resource_exhausted", "degraded"
	case errors.Is(err, domain.ErrRecoveryRequired):
		code, category = "authority_recovery_required", "degraded"
	case errors.Is(err, domain.ErrUnavailable), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		code, category, retryable = "authority_unavailable", "unavailable", true
	}
	return &rpc.Error{
		Code: code, Category: category, Message: "Realm Authority request failed",
		Domain: "authority", Operation: operation, Reason: code, Retryable: retryable,
		Details: map[string]any{},
	}
}
