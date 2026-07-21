// Package content adapts the public Application content protocol to the Content owner interface.
package content

import (
	applicationauth "ardents/internal/applicationapi/auth"
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
	PublishBlob(appcontent.PublishBlobCommand) (appcontent.Blob, error)
	GetBlob(string) (appcontent.Blob, bool)
	GetBlobPayload(string) ([]byte, error)
	FetchBlob(context.Context, string) (appcontent.Blob, error)
}

type Authorizer interface {
	Authorize(context.Context, http.Header, string) error
}

type Handler struct {
	store      Store
	authorizer Authorizer
}

func NewHandler(store Store, authorizer Authorizer) (*Handler, error) {
	if store == nil {
		return nil, fmt.Errorf("application content store is required")
	}
	if authorizer == nil {
		return nil, fmt.Errorf("application authorizer is required")
	}
	return &Handler{store: store, authorizer: authorizer}, nil
}

func NewHTTPHandler(store Store, authorizer Authorizer) (string, http.Handler, error) {
	handler, err := NewHandler(store, authorizer)
	if err != nil {
		return "", nil, err
	}
	path, httpHandler := applicationv1connect.NewContentServiceHandler(
		handler,
		connect.WithReadMaxBytes(applicationv1.MaxUnaryMessageBytes),
		connect.WithSendMaxBytes(applicationv1.MaxUnaryMessageBytes),
	)
	return path, httpHandler, nil
}

func (h *Handler) Put(ctx context.Context, req *connect.Request[applicationv1.PutContentRequest]) (*connect.Response[applicationv1.PutContentResponse], error) {
	if err := h.authorizer.Authorize(ctx, req.Header(), ActionPut); err != nil {
		return nil, mapAuthorizationError(ActionPut, err)
	}
	payload := req.Msg.GetPayload()
	if len(payload) == 0 {
		return nil, protocolError(applicationv1.ErrorCode_ERROR_CODE_INVALID_ARGUMENT, ActionPut, "content payload is required", false, connect.CodeInvalidArgument)
	}
	if len(payload) > applicationv1.MaxUnaryPayloadBytes {
		return nil, contentTooLarge(ActionPut)
	}
	mediaType := strings.TrimSpace(req.Msg.GetMediaType())
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	stored, err := h.store.PublishBlob(appcontent.PublishBlobCommand{
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
	if err := h.authorizer.Authorize(ctx, req.Header(), ActionGet); err != nil {
		return nil, mapAuthorizationError(ActionGet, err)
	}
	reference := req.Msg.GetReference()
	if reference == nil || reference.GetKind() != "blob" || strings.TrimSpace(reference.GetId()) == "" {
		return nil, protocolError(applicationv1.ErrorCode_ERROR_CODE_INVALID_ARGUMENT, ActionGet, "valid blob reference is required", false, connect.CodeInvalidArgument)
	}
	blob, found := h.store.GetBlob(reference.GetId())
	var payload []byte
	var payloadErr error
	if found {
		if blob.Size < 0 || blob.Size > applicationv1.MaxUnaryPayloadBytes {
			return nil, contentTooLarge(ActionGet)
		}
		payload, payloadErr = h.store.GetBlobPayload(reference.GetId())
		if payloadErr != nil && !errors.Is(payloadErr, appcontent.ErrBlobPayloadNotLocal) {
			return nil, mapStoreError(ActionGet, payloadErr, false)
		}
	}
	if !found || payloadErr != nil {
		var fetchErr error
		blob, fetchErr = h.store.FetchBlob(ctx, reference.GetId())
		if fetchErr != nil {
			return nil, mapStoreError(ActionGet, fetchErr, true)
		}
		if blob.Size < 0 || blob.Size > applicationv1.MaxUnaryPayloadBytes {
			return nil, contentTooLarge(ActionGet)
		}
		payload, payloadErr = h.store.GetBlobPayload(blob.ID)
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

func mapAuthorizationError(operation string, err error) error {
	var connectErr *connect.Error
	switch {
	case errors.Is(err, applicationauth.ErrUnauthenticated):
		return protocolError(applicationv1.ErrorCode_ERROR_CODE_UNAUTHENTICATED, operation, "application authentication required", false, connect.CodeUnauthenticated)
	case errors.Is(err, applicationauth.ErrForbidden):
		return protocolError(applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN, operation, "application action is forbidden", false, connect.CodePermissionDenied)
	case errors.As(err, &connectErr) && connectErr.Code() == connect.CodeUnauthenticated:
		return protocolError(applicationv1.ErrorCode_ERROR_CODE_UNAUTHENTICATED, operation, "application authentication required", false, connect.CodeUnauthenticated)
	case errors.As(err, &connectErr) && connectErr.Code() == connect.CodePermissionDenied:
		return protocolError(applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN, operation, "application action is forbidden", false, connect.CodePermissionDenied)
	default:
		return protocolError(applicationv1.ErrorCode_ERROR_CODE_INTERNAL, operation, "application authorization failed", false, connect.CodeInternal)
	}
}

func protocolError(code applicationv1.ErrorCode, operation, message string, retryable bool, connectCode connect.Code) error {
	result := connect.NewError(connectCode, errors.New(message))
	detail, err := connect.NewErrorDetail(&applicationv1.ApplicationError{
		Code: code, Operation: operation, Message: message, Retryable: retryable,
	})
	if err == nil {
		result.AddDetail(detail)
	}
	return result
}
