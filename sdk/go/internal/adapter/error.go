package adapter

import (
	stderrors "errors"

	sdkerrors "ardents/sdk/go/errors"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"

	"connectrpc.com/connect"
)

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
