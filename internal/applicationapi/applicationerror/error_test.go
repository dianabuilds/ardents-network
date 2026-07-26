package applicationerror_test

import (
	"testing"

	applicationerror "ardents/internal/applicationapi/applicationerror"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestSharedApplicationErrorPreservesWireIdentityAndFields(t *testing.T) {
	enum := applicationv1.ErrorCode_ERROR_CODE_UNAUTHENTICATED.Descriptor()
	require.Equal(t, protoreflect.FullName("ardents.application.v1.ErrorCode"), enum.FullName())
	value := enum.Values().ByName("ERROR_CODE_UNAUTHENTICATED")
	require.NotNil(t, value)
	require.Equal(t, protoreflect.EnumNumber(2), value.Number())

	message := (&applicationv1.ApplicationError{}).ProtoReflect().Descriptor()
	require.Equal(t, protoreflect.FullName("ardents.application.v1.ApplicationError"), message.FullName())
	for name, number := range map[protoreflect.Name]protoreflect.FieldNumber{
		"code": 1, "operation": 2, "message": 3, "retryable": 4, "details": 5,
	} {
		require.Equal(t, number, message.Fields().ByName(name).Number(), name)
	}
}

func TestSharedApplicationErrorConstructsTheExistingConnectDetail(t *testing.T) {
	err := applicationerror.ProtocolError(
		applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN,
		"application.content.get",
		"application action is forbidden",
		false,
		connect.CodePermissionDenied,
	)

	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	require.Len(t, connectErr.Details(), 1)
	value, detailErr := connectErr.Details()[0].Value()
	require.NoError(t, detailErr)
	detail, ok := value.(*applicationv1.ApplicationError)
	require.True(t, ok)
	require.Equal(t, applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN, detail.GetCode())
	require.Equal(t, "application.content.get", detail.GetOperation())
	require.Equal(t, "application action is forbidden", detail.GetMessage())
	require.False(t, detail.GetRetryable())
}
