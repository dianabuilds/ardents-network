package discovery

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	applicationerror "ardents/internal/applicationapi/applicationerror"
	applicationcall "ardents/internal/applicationapi/call"
	identityaccess "ardents/internal/identity/access"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"
)

type ServiceLocator interface {
	Resolve(Query) ([]Target, error)
}

type Handler struct {
	locator   ServiceLocator
	extractor applicationcall.Extractor
}

func NewHandler(locator ServiceLocator, extractor applicationcall.Extractor) (*Handler, error) {
	if locator == nil {
		return nil, fmt.Errorf("application discovery locator is required")
	}
	if !extractor.Valid() {
		return nil, fmt.Errorf("application admission extractor is required")
	}
	return &Handler{locator: locator, extractor: extractor}, nil
}

func NewHTTPHandler(locator ServiceLocator, extractor applicationcall.Extractor, interceptors ...connect.Interceptor) (string, http.Handler, error) {
	handler, err := NewHandler(locator, extractor)
	if err != nil {
		return "", nil, err
	}
	path, httpHandler := applicationv1connect.NewDiscoveryServiceHandler(
		handler,
		connect.WithReadMaxBytes(applicationv1.MaxUnaryMessageBytes),
		connect.WithSendMaxBytes(applicationv1.MaxUnaryMessageBytes),
		connect.WithInterceptors(interceptors...),
		connect.WithCodec(strictJSONCodec{name: "json"}),
		connect.WithCodec(strictJSONCodec{name: "json; charset=utf-8"}),
	)
	return path, httpHandler, nil
}

type strictJSONCodec struct {
	name string
}

func (c strictJSONCodec) Name() string {
	return c.name
}

func (strictJSONCodec) Marshal(message any) ([]byte, error) {
	protoMessage, ok := message.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("strict JSON codec requires a protobuf message, got %T", message)
	}
	return protojson.MarshalOptions{}.Marshal(protoMessage)
}

func (strictJSONCodec) Unmarshal(data []byte, message any) error {
	protoMessage, ok := message.(proto.Message)
	if !ok {
		return fmt.Errorf("strict JSON codec requires a protobuf message, got %T", message)
	}
	return protojson.UnmarshalOptions{DiscardUnknown: false}.Unmarshal(data, protoMessage)
}

func (h *Handler) Resolve(ctx context.Context, request *connect.Request[applicationv1.ResolveServiceRequest]) (*connect.Response[applicationv1.ResolveServiceResponse], error) {
	target, err := CanonicalizeResource(applicationv1connect.DiscoveryServiceResolveProcedure, request.Msg)
	if err != nil {
		return nil, invalidArgumentError()
	}
	_, err = h.admittedCall(ctx, target)
	if err != nil {
		return nil, err
	}
	query := Query{
		ServiceType:     request.Msg.GetServiceType(),
		AcceptedSchemes: append([]string(nil), request.Msg.GetAcceptedSchemes()...),
	}
	targets, err := h.locator.Resolve(query)
	if err != nil {
		return nil, mapLocatorError(err)
	}
	if !validTargetSet(query, targets) {
		return nil, mapLocatorError(ErrInternal)
	}
	responseTargets := make([]*applicationv1.ResolvedServiceTarget, 0, len(targets))
	for _, item := range targets {
		responseTargets = append(responseTargets, &applicationv1.ResolvedServiceTarget{
			ServiceId: item.ServiceID, Endpoint: item.Endpoint, Scheme: item.Scheme,
		})
	}
	return connect.NewResponse(&applicationv1.ResolveServiceResponse{Targets: responseTargets}), nil
}

func (h *Handler) admittedCall(ctx context.Context, target identityaccess.ResourceTarget) (applicationcall.Call, error) {
	admitted, ok := h.extractor.Extract(ctx)
	if !ok || admitted.Action() != ActionResolve {
		return applicationcall.Call{}, applicationerror.ProtocolError(
			applicationv1.ErrorCode_ERROR_CODE_UNAUTHENTICATED,
			ActionResolve, "application authentication required", false, connect.CodeUnauthenticated,
		)
	}
	if !admitted.IsPrincipal() || admitted.ResourceNode() != admitted.Node() ||
		!admitted.ResourceOwner().IsNone() || admitted.ResourceKind() != string(target.Kind) ||
		admitted.ResourceID() != target.ID {
		return applicationcall.Call{}, applicationerror.ProtocolError(
			applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN,
			ActionResolve, "application action is forbidden", false, connect.CodePermissionDenied,
		)
	}
	return admitted, nil
}

func invalidArgumentError() error {
	return applicationerror.ProtocolError(
		applicationv1.ErrorCode_ERROR_CODE_INVALID_ARGUMENT,
		ActionResolve, "invalid application discovery request", false, connect.CodeInvalidArgument,
	)
}

func mapLocatorError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidArgument):
		return invalidArgumentError()
	case errors.Is(err, ErrNotFound):
		return applicationerror.ProtocolError(
			applicationv1.ErrorCode_ERROR_CODE_NOT_FOUND,
			ActionResolve, "service was not found", false, connect.CodeNotFound,
		)
	case errors.Is(err, ErrUnavailable):
		return applicationerror.ProtocolError(
			applicationv1.ErrorCode_ERROR_CODE_UNAVAILABLE,
			ActionResolve, "application discovery is unavailable", true, connect.CodeUnavailable,
		)
	default:
		return applicationerror.ProtocolError(
			applicationv1.ErrorCode_ERROR_CODE_INTERNAL,
			ActionResolve, "application discovery failed", false, connect.CodeInternal,
		)
	}
}
