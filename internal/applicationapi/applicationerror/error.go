// Package applicationerror constructs the shared structured errors returned by
// protected Application services. It does not own product-specific error policy.
package applicationerror

import (
	"errors"

	applicationv1 "ardents/sdk/go/protocol/applicationv1"

	"connectrpc.com/connect"
)

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
