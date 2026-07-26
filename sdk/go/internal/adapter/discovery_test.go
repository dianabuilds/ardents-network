package adapter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"ardents/sdk/go/discovery"
	sdkerrors "ardents/sdk/go/errors"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryAdapterMapsDomainTypesAndRefreshesSessionOnce(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	node, signer := testIdentity(t, now)
	var begins atomic.Int32
	manager := testSessionManager(successfulAuthentication(node, signer.principal, now, &begins), signer, node, now)
	var calls atomic.Int32
	path, handler := applicationv1connect.NewDiscoveryServiceHandler(discoveryHandlerFunc(
		func(_ context.Context, request *connect.Request[applicationv1.ResolveServiceRequest]) (*connect.Response[applicationv1.ResolveServiceResponse], error) {
			require.Equal(t, "echo", request.Msg.GetServiceType())
			require.Equal(t, []string{"https", "tcp"}, request.Msg.GetAcceptedSchemes())
			if calls.Add(1) == 1 {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("expired"))
			}
			return connect.NewResponse(&applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{{
				ServiceId: "svc.echo", Endpoint: "https://10.20.30.40:8443", Scheme: "https",
			}}}), nil
		},
	))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	service := NewDiscovery(server.Client(), server.URL, connect.WithInterceptors(NewSessionInterceptor(manager)))

	targets, err := service.Resolve(context.Background(), discovery.Query{
		ServiceType: "echo", AcceptedSchemes: []discovery.Scheme{discovery.SchemeHTTPS, discovery.SchemeTCP},
	})

	require.NoError(t, err)
	require.Equal(t, []discovery.Target{{
		ServiceID: "svc.echo", Endpoint: "https://10.20.30.40:8443", Scheme: discovery.SchemeHTTPS,
	}}, targets)
	require.Equal(t, int32(2), calls.Load())
	require.Equal(t, int32(2), begins.Load())
}

func TestDiscoveryAdapterPreservesStableTypedErrors(t *testing.T) {
	tests := []struct {
		wire applicationv1.ErrorCode
		code sdkerrors.Code
	}{
		{applicationv1.ErrorCode_ERROR_CODE_INVALID_ARGUMENT, sdkerrors.InvalidArgument},
		{applicationv1.ErrorCode_ERROR_CODE_UNAUTHENTICATED, sdkerrors.Unauthenticated},
		{applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN, sdkerrors.Forbidden},
		{applicationv1.ErrorCode_ERROR_CODE_NOT_FOUND, sdkerrors.NotFound},
		{applicationv1.ErrorCode_ERROR_CODE_UNAVAILABLE, sdkerrors.Unavailable},
		{applicationv1.ErrorCode_ERROR_CODE_INTERNAL, sdkerrors.Internal},
	}
	for _, test := range tests {
		t.Run(test.wire.String(), func(t *testing.T) {
			path, handler := applicationv1connect.NewDiscoveryServiceHandler(discoveryHandlerFunc(
				func(context.Context, *connect.Request[applicationv1.ResolveServiceRequest]) (*connect.Response[applicationv1.ResolveServiceResponse], error) {
					remote := connect.NewError(connect.CodeInternal, errors.New("redacted"))
					detail, err := connect.NewErrorDetail(&applicationv1.ApplicationError{
						Code: test.wire, Operation: "application.discovery.resolve",
						Message: "stable discovery failure",
					})
					require.NoError(t, err)
					remote.AddDetail(detail)
					return nil, remote
				},
			))
			mux := http.NewServeMux()
			mux.Handle(path, handler)
			server := httptest.NewServer(mux)
			t.Cleanup(server.Close)

			_, err := NewDiscovery(server.Client(), server.URL).Resolve(context.Background(), discovery.Query{
				ServiceType: "echo", AcceptedSchemes: []discovery.Scheme{discovery.SchemeHTTPS},
			})

			var sdkErr *sdkerrors.Error
			require.ErrorAs(t, err, &sdkErr)
			require.Equal(t, test.code, sdkErr.Code)
			require.Equal(t, "application.discovery.resolve", sdkErr.Operation)
			require.Equal(t, "stable discovery failure", sdkErr.Message)
		})
	}
}

type discoveryHandlerFunc func(context.Context, *connect.Request[applicationv1.ResolveServiceRequest]) (*connect.Response[applicationv1.ResolveServiceResponse], error)

func (f discoveryHandlerFunc) Resolve(ctx context.Context, request *connect.Request[applicationv1.ResolveServiceRequest]) (*connect.Response[applicationv1.ResolveServiceResponse], error) {
	return f(ctx, request)
}
