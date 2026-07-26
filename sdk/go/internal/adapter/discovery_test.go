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
	"google.golang.org/protobuf/encoding/protowire"
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

func TestDiscoveryAdapterNeverRefreshesSessionAfterForbiddenOrNotFound(t *testing.T) {
	tests := []struct {
		name        string
		connect     connect.Code
		sdk         sdkerrors.Code
		wire        applicationv1.ErrorCode
		retryable   bool
		wireMessage string
	}{
		{
			name: "forbidden", connect: connect.CodePermissionDenied, sdk: sdkerrors.Forbidden,
			wire: applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN, wireMessage: "application action is forbidden",
		},
		{
			name: "not found", connect: connect.CodeNotFound, sdk: sdkerrors.NotFound,
			wire: applicationv1.ErrorCode_ERROR_CODE_NOT_FOUND, wireMessage: "service was not found",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := time.Unix(1_900_000_000, 0).UTC()
			node, signer := testIdentity(t, now)
			var begins atomic.Int32
			manager := testSessionManager(successfulAuthentication(node, signer.principal, now, &begins), signer, node, now)
			var calls atomic.Int32
			path, handler := applicationv1connect.NewDiscoveryServiceHandler(discoveryHandlerFunc(
				func(context.Context, *connect.Request[applicationv1.ResolveServiceRequest]) (*connect.Response[applicationv1.ResolveServiceResponse], error) {
					calls.Add(1)
					remote := connect.NewError(test.connect, errors.New("redacted"))
					detail, err := connect.NewErrorDetail(&applicationv1.ApplicationError{
						Code: test.wire, Operation: "application.discovery.resolve",
						Message: test.wireMessage, Retryable: test.retryable,
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
			service := NewDiscovery(server.Client(), server.URL, connect.WithInterceptors(NewSessionInterceptor(manager)))

			_, err := service.Resolve(context.Background(), discovery.Query{
				ServiceType: "echo", AcceptedSchemes: []discovery.Scheme{discovery.SchemeHTTPS},
			})

			var sdkErr *sdkerrors.Error
			require.ErrorAs(t, err, &sdkErr)
			require.Equal(t, test.sdk, sdkErr.Code)
			require.Equal(t, int32(1), calls.Load())
			require.Equal(t, int32(1), begins.Load())
		})
	}
}

func TestDiscoveryAdapterRejectsInvalidResponses(t *testing.T) {
	validTarget := func(serviceID, endpoint, scheme string) *applicationv1.ResolvedServiceTarget {
		return &applicationv1.ResolvedServiceTarget{
			ServiceId: serviceID, Endpoint: endpoint, Scheme: scheme,
		}
	}
	tests := []struct {
		name     string
		schemes  []discovery.Scheme
		options  []connect.ClientOption
		response func() *applicationv1.ResolveServiceResponse
	}{
		{
			name:    "empty target set",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			response: func() *applicationv1.ResolveServiceResponse {
				return &applicationv1.ResolveServiceResponse{}
			},
		},
		{
			name:    "over cap",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			response: func() *applicationv1.ResolveServiceResponse {
				targets := make([]*applicationv1.ResolvedServiceTarget, 0, 9)
				for index := 0; index < 9; index++ {
					targets = append(targets, validTarget(
						"svc."+string(rune('a'+index)), "https://10.20.30.40:8443", "https",
					))
				}
				return &applicationv1.ResolveServiceResponse{Targets: targets}
			},
		},
		{
			name:    "nil target",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			response: func() *applicationv1.ResolveServiceResponse {
				return &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{nil}}
			},
		},
		{
			name:    "noncanonical service ID",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			response: func() *applicationv1.ResolveServiceResponse {
				return &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{
					validTarget(" svc.echo", "https://10.20.30.40:8443", "https"),
				}}
			},
		},
		{
			name:    "unsupported scheme",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			response: func() *applicationv1.ResolveServiceResponse {
				return &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{
					validTarget("svc.echo", "quic://10.20.30.40:8443", "quic"),
				}}
			},
		},
		{
			name:    "scheme not accepted by caller",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			response: func() *applicationv1.ResolveServiceResponse {
				return &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{
					validTarget("svc.echo", "http://10.20.30.40:8080", "http"),
				}}
			},
		},
		{
			name:    "declared scheme differs from endpoint",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS, discovery.SchemeHTTP},
			response: func() *applicationv1.ResolveServiceResponse {
				return &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{
					validTarget("svc.echo", "http://10.20.30.40:8080", "https"),
				}}
			},
		},
		{
			name:    "DNS endpoint",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			response: func() *applicationv1.ResolveServiceResponse {
				return &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{
					validTarget("svc.echo", "https://service.internal:8443", "https"),
				}}
			},
		},
		{
			name:    "credential-bearing endpoint",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			response: func() *applicationv1.ResolveServiceResponse {
				return &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{
					validTarget("svc.echo", "https://user:secret@10.20.30.40:8443", "https"),
				}}
			},
		},
		{
			name:    "fragment-bearing endpoint",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			response: func() *applicationv1.ResolveServiceResponse {
				return &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{
					validTarget("svc.echo", "https://10.20.30.40:8443/#private", "https"),
				}}
			},
		},
		{
			name:    "non-ASCII endpoint",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			response: func() *applicationv1.ResolveServiceResponse {
				return &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{
					validTarget("svc.echo", "https://10.20.30.40:8443/café", "https"),
				}}
			},
		},
		{
			name:    "endpoint without explicit port",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			response: func() *applicationv1.ResolveServiceResponse {
				return &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{
					validTarget("svc.echo", "https://10.20.30.40/api", "https"),
				}}
			},
		},
		{
			name:    "unsafe endpoint host",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			response: func() *applicationv1.ResolveServiceResponse {
				return &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{
					validTarget("svc.echo", "https://127.0.0.1:8443", "https"),
				}}
			},
		},
		{
			name:    "duplicate pair",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			response: func() *applicationv1.ResolveServiceResponse {
				target := validTarget("svc.echo", "https://10.20.30.40:8443", "https")
				return &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{
					target, validTarget(target.GetServiceId(), target.GetEndpoint(), target.GetScheme()),
				}}
			},
		},
		{
			name:    "scheme preference order",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS, discovery.SchemeHTTP},
			response: func() *applicationv1.ResolveServiceResponse {
				return &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{
					validTarget("svc.echo", "http://10.20.30.40:8080", "http"),
					validTarget("svc.echo", "https://10.20.30.40:8443", "https"),
				}}
			},
		},
		{
			name:    "service ID order",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			response: func() *applicationv1.ResolveServiceResponse {
				return &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{
					validTarget("svc.z", "https://10.20.30.40:8443", "https"),
					validTarget("svc.a", "https://10.20.30.40:8443", "https"),
				}}
			},
		},
		{
			name:    "endpoint byte order",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			response: func() *applicationv1.ResolveServiceResponse {
				return &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{
					validTarget("svc.echo", "https://10.20.30.40:8444", "https"),
					validTarget("svc.echo", "https://10.20.30.40:8443", "https"),
				}}
			},
		},
		{
			name:    "unknown response field",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			response: func() *applicationv1.ResolveServiceResponse {
				response := &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{
					validTarget("svc.echo", "https://10.20.30.40:8443", "https"),
				}}
				response.ProtoReflect().SetUnknown(
					protowire.AppendVarint(protowire.AppendTag(nil, 99, protowire.VarintType), 1),
				)
				return response
			},
		},
		{
			name:    "JSON option cannot discard unknown response field",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			options: []connect.ClientOption{connect.WithProtoJSON()},
			response: func() *applicationv1.ResolveServiceResponse {
				response := &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{
					validTarget("svc.echo", "https://10.20.30.40:8443", "https"),
				}}
				response.ProtoReflect().SetUnknown(
					protowire.AppendVarint(protowire.AppendTag(nil, 99, protowire.VarintType), 1),
				)
				return response
			},
		},
		{
			name:    "unknown target field",
			schemes: []discovery.Scheme{discovery.SchemeHTTPS},
			response: func() *applicationv1.ResolveServiceResponse {
				target := validTarget("svc.echo", "https://10.20.30.40:8443", "https")
				target.ProtoReflect().SetUnknown(
					protowire.AppendVarint(protowire.AppendTag(nil, 99, protowire.VarintType), 1),
				)
				return &applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{target}}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path, handler := applicationv1connect.NewDiscoveryServiceHandler(discoveryHandlerFunc(
				func(context.Context, *connect.Request[applicationv1.ResolveServiceRequest]) (*connect.Response[applicationv1.ResolveServiceResponse], error) {
					return connect.NewResponse(test.response()), nil
				},
			))
			mux := http.NewServeMux()
			mux.Handle(path, handler)
			server := httptest.NewServer(mux)
			t.Cleanup(server.Close)

			_, err := NewDiscovery(server.Client(), server.URL, test.options...).Resolve(context.Background(), discovery.Query{
				ServiceType: "echo", AcceptedSchemes: test.schemes,
			})

			var sdkErr *sdkerrors.Error
			require.ErrorAs(t, err, &sdkErr)
			require.Equal(t, sdkerrors.Internal, sdkErr.Code)
			require.Equal(t, "application.discovery.resolve", sdkErr.Operation)
			require.Equal(t, "application discovery response is invalid", sdkErr.Message)
		})
	}
}

func TestDiscoveryAdapterAcceptsStrictlyOrderedExactDistinctTargets(t *testing.T) {
	path, handler := applicationv1connect.NewDiscoveryServiceHandler(discoveryHandlerFunc(
		func(context.Context, *connect.Request[applicationv1.ResolveServiceRequest]) (*connect.Response[applicationv1.ResolveServiceResponse], error) {
			return connect.NewResponse(&applicationv1.ResolveServiceResponse{Targets: []*applicationv1.ResolvedServiceTarget{
				{ServiceId: "svc.a", Endpoint: "https://10.20.30.40:8443", Scheme: "https"},
				{ServiceId: "svc.b", Endpoint: "https://10.20.30.40:8443", Scheme: "https"},
				{ServiceId: "svc.a", Endpoint: "http://10.20.30.41:8080", Scheme: "http"},
			}}), nil
		},
	))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	targets, err := NewDiscovery(server.Client(), server.URL).Resolve(context.Background(), discovery.Query{
		ServiceType: "echo", AcceptedSchemes: []discovery.Scheme{
			discovery.SchemeHTTPS, discovery.SchemeHTTP,
		},
	})

	require.NoError(t, err)
	require.Equal(t, []discovery.Target{
		{ServiceID: "svc.a", Endpoint: "https://10.20.30.40:8443", Scheme: discovery.SchemeHTTPS},
		{ServiceID: "svc.b", Endpoint: "https://10.20.30.40:8443", Scheme: discovery.SchemeHTTPS},
		{ServiceID: "svc.a", Endpoint: "http://10.20.30.41:8080", Scheme: discovery.SchemeHTTP},
	}, targets)
}

type discoveryHandlerFunc func(context.Context, *connect.Request[applicationv1.ResolveServiceRequest]) (*connect.Response[applicationv1.ResolveServiceResponse], error)

func (f discoveryHandlerFunc) Resolve(ctx context.Context, request *connect.Request[applicationv1.ResolveServiceRequest]) (*connect.Response[applicationv1.ResolveServiceResponse], error) {
	return f(ctx, request)
}
