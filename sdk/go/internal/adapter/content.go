package adapter

import (
	"context"
	stderrors "errors"
	"strings"

	"ardents/sdk/go/content"
	sdkerrors "ardents/sdk/go/errors"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"

	"connectrpc.com/connect"
)

type Content struct {
	client applicationv1connect.ContentServiceClient
}

func NewContent(httpClient connect.HTTPClient, endpoint string, options ...connect.ClientOption) *Content {
	clientOptions := []connect.ClientOption{
		connect.WithReadMaxBytes(applicationv1.MaxUnaryMessageBytes),
		connect.WithSendMaxBytes(applicationv1.MaxUnaryMessageBytes),
	}
	clientOptions = append(clientOptions, options...)
	return &Content{
		client: applicationv1connect.NewContentServiceClient(
			httpClient,
			strings.TrimRight(endpoint, "/"),
			clientOptions...,
		),
	}
}

func (c *Content) Put(ctx context.Context, payload []byte, options ...content.PutOption) (content.Reference, error) {
	configured := content.PutOptions{}
	for _, option := range options {
		if option != nil {
			option(&configured)
		}
	}
	req := connect.NewRequest(&applicationv1.PutContentRequest{Payload: append([]byte(nil), payload...), MediaType: configured.MediaType})
	response, err := c.client.Put(ctx, req)
	if err != nil {
		return content.Reference{}, mapError(err)
	}
	reference := response.Msg.GetReference()
	if reference == nil {
		return content.Reference{}, &sdkerrors.Error{Code: sdkerrors.Internal, Operation: "application.content.put", Message: "content response has no reference"}
	}
	return content.Reference{Kind: reference.GetKind(), ID: reference.GetId()}, nil
}

func (c *Content) Get(ctx context.Context, reference content.Reference) ([]byte, error) {
	req := connect.NewRequest(&applicationv1.GetContentRequest{Reference: &applicationv1.ContentReference{Kind: reference.Kind, Id: reference.ID}})
	response, err := c.client.Get(ctx, req)
	if err != nil {
		return nil, mapError(err)
	}
	return append([]byte(nil), response.Msg.GetPayload()...), nil
}

func mapError(err error) error {
	var sdkErr *sdkerrors.Error
	if stderrors.As(err, &sdkErr) {
		return sdkErr
	}
	var connectErr *connect.Error
	if !stderrors.As(err, &connectErr) {
		return &sdkerrors.Error{Code: sdkerrors.Internal, Message: "application request failed"}
	}
	for _, detail := range connectErr.Details() {
		value, valueErr := detail.Value()
		if valueErr != nil {
			continue
		}
		if applicationErr, ok := value.(*applicationv1.ApplicationError); ok {
			return &sdkerrors.Error{
				Code: code(applicationErr.GetCode()), Operation: applicationErr.GetOperation(),
				Message: applicationErr.GetMessage(), Retryable: applicationErr.GetRetryable(),
				Details: cloneDetails(applicationErr.GetDetails()),
			}
		}
	}
	return &sdkerrors.Error{Code: connectCode(connectErr.Code()), Message: "application request failed"}
}

func cloneDetails(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func code(value applicationv1.ErrorCode) sdkerrors.Code {
	switch value {
	case applicationv1.ErrorCode_ERROR_CODE_INVALID_ARGUMENT:
		return sdkerrors.InvalidArgument
	case applicationv1.ErrorCode_ERROR_CODE_UNAUTHENTICATED:
		return sdkerrors.Unauthenticated
	case applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN:
		return sdkerrors.Forbidden
	case applicationv1.ErrorCode_ERROR_CODE_NOT_FOUND:
		return sdkerrors.NotFound
	case applicationv1.ErrorCode_ERROR_CODE_CONFLICT:
		return sdkerrors.Conflict
	case applicationv1.ErrorCode_ERROR_CODE_UNAVAILABLE:
		return sdkerrors.Unavailable
	case applicationv1.ErrorCode_ERROR_CODE_INTEGRITY_FAILED:
		return sdkerrors.IntegrityFailed
	case applicationv1.ErrorCode_ERROR_CODE_RESOURCE_EXHAUSTED:
		return sdkerrors.ResourceExhausted
	default:
		return sdkerrors.Internal
	}
}

func connectCode(value connect.Code) sdkerrors.Code {
	switch value {
	case connect.CodeInvalidArgument:
		return sdkerrors.InvalidArgument
	case connect.CodeResourceExhausted:
		return sdkerrors.ResourceExhausted
	case connect.CodeUnauthenticated:
		return sdkerrors.Unauthenticated
	case connect.CodePermissionDenied:
		return sdkerrors.Forbidden
	case connect.CodeNotFound:
		return sdkerrors.NotFound
	case connect.CodeAlreadyExists:
		return sdkerrors.Conflict
	case connect.CodeUnavailable:
		return sdkerrors.Unavailable
	case connect.CodeDataLoss:
		return sdkerrors.IntegrityFailed
	default:
		return sdkerrors.Internal
	}
}
