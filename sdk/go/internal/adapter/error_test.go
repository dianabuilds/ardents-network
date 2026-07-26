package adapter

import (
	"testing"

	sdkerrors "ardents/sdk/go/errors"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestSharedApplicationErrorMappingPreservesTypedSDKCodesAndDetails(t *testing.T) {
	tests := []struct {
		wire applicationv1.ErrorCode
		sdk  sdkerrors.Code
	}{
		{applicationv1.ErrorCode_ERROR_CODE_INVALID_ARGUMENT, sdkerrors.InvalidArgument},
		{applicationv1.ErrorCode_ERROR_CODE_UNAUTHENTICATED, sdkerrors.Unauthenticated},
		{applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN, sdkerrors.Forbidden},
		{applicationv1.ErrorCode_ERROR_CODE_NOT_FOUND, sdkerrors.NotFound},
		{applicationv1.ErrorCode_ERROR_CODE_CONFLICT, sdkerrors.Conflict},
		{applicationv1.ErrorCode_ERROR_CODE_UNAVAILABLE, sdkerrors.Unavailable},
		{applicationv1.ErrorCode_ERROR_CODE_INTEGRITY_FAILED, sdkerrors.IntegrityFailed},
		{applicationv1.ErrorCode_ERROR_CODE_RESOURCE_EXHAUSTED, sdkerrors.ResourceExhausted},
		{applicationv1.ErrorCode_ERROR_CODE_INTERNAL, sdkerrors.Internal},
	}
	for _, test := range tests {
		t.Run(test.wire.String(), func(t *testing.T) {
			remote := connect.NewError(connect.CodeInternal, nil)
			detail, err := connect.NewErrorDetail(&applicationv1.ApplicationError{
				Code: test.wire, Operation: "application.content.get", Message: "stable",
				Retryable: true, Details: map[string]string{"key": "value"},
			})
			require.NoError(t, err)
			remote.AddDetail(detail)

			mapped := mapError(remote)

			var sdkErr *sdkerrors.Error
			require.ErrorAs(t, mapped, &sdkErr)
			require.Equal(t, test.sdk, sdkErr.Code)
			require.Equal(t, "application.content.get", sdkErr.Operation)
			require.Equal(t, "stable", sdkErr.Message)
			require.True(t, sdkErr.Retryable)
			require.Equal(t, map[string]string{"key": "value"}, sdkErr.Details)
		})
	}
}
