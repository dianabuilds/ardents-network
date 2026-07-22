// Package content adapts the public Application content protocol to the Content owner interface.
package content

import (
	applicationcall "ardents/internal/applicationapi/call"
	appcontent "ardents/internal/content"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/connect"
)

const (
	ActionPut = "application.content.put"
	ActionGet = "application.content.get"
)

type Store interface {
	PublishBlob(applicationcall.Call, appcontent.PublishBlobCommand) (appcontent.Blob, error)
	GetBlob(applicationcall.Call, string) (appcontent.Blob, bool)
	GetBlobPayload(applicationcall.Call, string) ([]byte, error)
	FetchBlob(context.Context, applicationcall.Call, string) (appcontent.Blob, error)
}

type Handler struct {
	store     Store
	extractor applicationcall.Extractor
}

func NewHandler(store Store, extractor applicationcall.Extractor) (*Handler, error) {
	if store == nil {
		return nil, fmt.Errorf("application content store is required")
	}
	if !extractor.Valid() {
		return nil, fmt.Errorf("application admission extractor is required")
	}
	return &Handler{store: store, extractor: extractor}, nil
}

func NewHTTPHandler(store Store, extractor applicationcall.Extractor, interceptors ...connect.Interceptor) (string, http.Handler, error) {
	handler, err := NewHandler(store, extractor)
	if err != nil {
		return "", nil, err
	}
	path, httpHandler := applicationv1connect.NewContentServiceHandler(
		handler,
		connect.WithReadMaxBytes(applicationv1.MaxUnaryMessageBytes),
		connect.WithSendMaxBytes(applicationv1.MaxUnaryMessageBytes),
		connect.WithInterceptors(interceptors...),
	)
	return path, httpHandler, nil
}

func (h *Handler) Put(ctx context.Context, req *connect.Request[applicationv1.PutContentRequest]) (*connect.Response[applicationv1.PutContentResponse], error) {
	target, err := CanonicalizeResource(applicationv1connect.ContentServicePutProcedure, req.Msg)
	if err != nil {
		return nil, mapTargetError(ActionPut, err)
	}
	admitted, err := h.admittedCall(ctx, ActionPut, target)
	if err != nil {
		return nil, err
	}
	payload := req.Msg.GetPayload()
	mediaType := strings.TrimSpace(req.Msg.GetMediaType())
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	stored, err := h.store.PublishBlob(admitted, appcontent.PublishBlobCommand{
		Blob: appcontent.Blob{MediaType: mediaType}, Payload: append([]byte(nil), payload...),
	})
	if err != nil {
		return nil, mapStoreError(ActionPut, err, false)
	}
	return connect.NewResponse(&applicationv1.PutContentResponse{
		Reference: &applicationv1.ContentReference{Kind: "blob", Id: stored.ID},
		Size:      stored.Size, MediaType: stored.MediaType,
	}), nil
}

func (h *Handler) Get(ctx context.Context, req *connect.Request[applicationv1.GetContentRequest]) (*connect.Response[applicationv1.GetContentResponse], error) {
	target, err := CanonicalizeResource(applicationv1connect.ContentServiceGetProcedure, req.Msg)
	if err != nil {
		return nil, mapTargetError(ActionGet, err)
	}
	admitted, err := h.admittedCall(ctx, ActionGet, target)
	if err != nil {
		return nil, err
	}
	reference := req.Msg.GetReference()
	blob, found := h.store.GetBlob(admitted, reference.GetId())
	var payload []byte
	var payloadErr error
	if found {
		if blob.Size < 0 || blob.Size > applicationv1.MaxUnaryPayloadBytes {
			return nil, contentTooLarge(ActionGet)
		}
		payload, payloadErr = h.store.GetBlobPayload(admitted, reference.GetId())
		if payloadErr != nil && !errors.Is(payloadErr, appcontent.ErrBlobPayloadNotLocal) {
			return nil, mapStoreError(ActionGet, payloadErr, false)
		}
	}
	if !found || payloadErr != nil {
		var fetchErr error
		blob, fetchErr = h.store.FetchBlob(ctx, admitted, reference.GetId())
		if fetchErr != nil {
			return nil, mapStoreError(ActionGet, fetchErr, true)
		}
		if blob.Size < 0 || blob.Size > applicationv1.MaxUnaryPayloadBytes {
			return nil, contentTooLarge(ActionGet)
		}
		payload, payloadErr = h.store.GetBlobPayload(admitted, blob.ID)
	}
	if payloadErr != nil {
		return nil, mapStoreError(ActionGet, payloadErr, false)
	}
	if len(payload) > applicationv1.MaxUnaryPayloadBytes {
		return nil, contentTooLarge(ActionGet)
	}
	if err := appcontent.VerifyBlobPayload(blob, payload); err != nil {
		return nil, protocolError(applicationv1.ErrorCode_ERROR_CODE_INTEGRITY_FAILED, ActionGet, "content integrity verification failed", false, connect.CodeDataLoss)
	}
	return connect.NewResponse(&applicationv1.GetContentResponse{
		Payload: append([]byte(nil), payload...), Size: blob.Size, MediaType: blob.MediaType,
	}), nil
}

func (h *Handler) admittedCall(ctx context.Context, action string, target ResourceTarget) (applicationcall.Call, error) {
	admitted, ok := h.extractor.Extract(ctx)
	if !ok || admitted.Action() != action {
		return applicationcall.Call{}, protocolError(applicationv1.ErrorCode_ERROR_CODE_UNAUTHENTICATED, action, "application authentication required", false, connect.CodeUnauthenticated)
	}
	if !admitted.IsPrincipal() || admitted.Actor() != admitted.Effective() || admitted.ResourceNode() != admitted.Node() ||
		admitted.ResourceOwner() != admitted.Effective() || admitted.ResourceKind() != target.Kind || admitted.ResourceID() != target.ID {
		return applicationcall.Call{}, protocolError(applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN, action, "application action is forbidden", false, connect.CodePermissionDenied)
	}
	return admitted, nil
}

func mapTargetError(operation string, err error) error {
	if errors.Is(err, ErrPayloadTooLarge) {
		return contentTooLarge(operation)
	}
	return protocolError(applicationv1.ErrorCode_ERROR_CODE_INVALID_ARGUMENT, operation, "invalid application content request", false, connect.CodeInvalidArgument)
}

func contentTooLarge(operation string) error {
	return protocolError(applicationv1.ErrorCode_ERROR_CODE_RESOURCE_EXHAUSTED, operation, "content payload exceeds the unary limit", false, connect.CodeResourceExhausted)
}

func mapStoreError(operation string, err error, retryable bool) error {
	switch {
	case errors.Is(err, appcontent.ErrBlobNotFound):
		return protocolError(applicationv1.ErrorCode_ERROR_CODE_NOT_FOUND, operation, "content not found", false, connect.CodeNotFound)
	case errors.Is(err, appcontent.ErrStoreUnavailable):
		return protocolError(applicationv1.ErrorCode_ERROR_CODE_UNAVAILABLE, operation, "content is unavailable", true, connect.CodeUnavailable)
	case errors.Is(err, appcontent.ErrBlobIntegrity):
		return protocolError(applicationv1.ErrorCode_ERROR_CODE_INTEGRITY_FAILED, operation, "content integrity verification failed", false, connect.CodeDataLoss)
	default:
		return protocolError(applicationv1.ErrorCode_ERROR_CODE_INTERNAL, operation, "content operation failed", retryable, connect.CodeInternal)
	}
}

func ProtocolError(code applicationv1.ErrorCode, operation, message string, retryable bool, connectCode connect.Code) error {
	result := connect.NewError(connectCode, errors.New(message))
	detail, err := connect.NewErrorDetail(&applicationv1.ApplicationError{
		Code: code, Operation: operation, Message: message, Retryable: retryable,
	})
	if err == nil {
		result.AddDetail(detail)
	}
	return result
}

func protocolError(code applicationv1.ErrorCode, operation, message string, retryable bool, connectCode connect.Code) error {
	return ProtocolError(code, operation, message, retryable, connectCode)
}
