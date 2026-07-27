package authority

import (
	"context"
	"errors"

	domain "ardents/internal/authority"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

type Service interface {
	CreateOrReopen(context.Context, domain.Command, domain.CreateRequest) (domain.CreateResult, error)
	Inspect(context.Context, domain.Command, domain.InspectRequest) (domain.Status, error)
	Readiness() domain.Status
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
