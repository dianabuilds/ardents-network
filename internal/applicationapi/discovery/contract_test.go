package discovery_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	applicationadmission "ardents/internal/applicationapi/admission"
	applicationcall "ardents/internal/applicationapi/call"
	applicationdiscovery "ardents/internal/applicationapi/discovery"
	discoverytruth "ardents/internal/discovery"
	discoveryrecord "ardents/internal/discovery/records"
	identityaccess "ardents/internal/identity/access"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"
	"ardents/tests/testkit"
	"google.golang.org/protobuf/encoding/protowire"
)

func TestDiscoveryResolveCrossesRealAdmittedApplicationContract(t *testing.T) {
	// Keep the Node-side contract binary on the generated Application wire
	// client. The internal and SDK identity protobuf registries are deliberately
	// isolated package graphs; SDK domain/adapter coverage lives under
	// sdk/go/internal/adapter and uses this exact generated wire contract.
	record, trustRegistry := testkit.TrustedNetworkPublishedService(t, "svc.echo", "echo", "https://10.20.30.40:8443")
	trust := discoverytruth.NewTrustEvaluator(trustRegistry)
	store := discoverytruth.NewWithTrust("", trust)
	_, err := store.Import(record, discoveryrecord.Imported)
	require.NoError(t, err)
	truth, err := applicationdiscovery.NewMaintainedTruth(store, trust)
	require.NoError(t, err)
	locator, err := applicationdiscovery.NewLocator(truth)
	require.NoError(t, err)
	client := newAdmittedDiscoveryClient(t, locator, []identityaccess.Action{applicationdiscovery.ActionResolve})

	response, err := client.Resolve(context.Background(), connect.NewRequest(&applicationv1.ResolveServiceRequest{
		ServiceType: "echo", AcceptedSchemes: []string{"https"},
	}))

	require.NoError(t, err)
	require.Equal(t, []*applicationv1.ResolvedServiceTarget{{
		ServiceId: "svc.echo", Endpoint: "https://10.20.30.40:8443", Scheme: "https",
	}}, response.Msg.GetTargets())
}

func TestDiscoveryAdmissionDenialDoesNotInvokeLocator(t *testing.T) {
	locator := &locatorStub{targets: []applicationdiscovery.Target{{
		ServiceID: "svc.echo", Endpoint: "https://10.20.30.40:8443", Scheme: "https",
	}}}
	client := newAdmittedDiscoveryClient(t, locator, []identityaccess.Action{"application.content.get"})

	_, err := client.Resolve(context.Background(), connect.NewRequest(&applicationv1.ResolveServiceRequest{
		ServiceType: "echo", AcceptedSchemes: []string{"https"},
	}))

	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	require.Zero(t, locator.calls)
	require.Equal(t, applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN, discoveryApplicationError(t, err).GetCode())
}

func TestDiscoveryMissingSessionIsTypedUnauthenticatedBeforeLocator(t *testing.T) {
	locator := &locatorStub{}
	client := newDiscoveryClient(t, locator, []identityaccess.Action{applicationdiscovery.ActionResolve}, false)

	_, err := client.Resolve(context.Background(), connect.NewRequest(&applicationv1.ResolveServiceRequest{
		ServiceType: "echo", AcceptedSchemes: []string{"https"},
	}))

	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.Zero(t, locator.calls)
	require.Equal(t, applicationv1.ErrorCode_ERROR_CODE_UNAUTHENTICATED, discoveryApplicationError(t, err).GetCode())
}

func TestDiscoveryMalformedAndUnknownFieldsFailBeforeLocator(t *testing.T) {
	locator := &locatorStub{}
	client := newAdmittedDiscoveryClient(t, locator, []identityaccess.Action{applicationdiscovery.ActionResolve})
	requests := map[string]*applicationv1.ResolveServiceRequest{
		"missing service type": {AcceptedSchemes: []string{"https"}},
		"padded service type":  {ServiceType: " echo", AcceptedSchemes: []string{"https"}},
		"missing schemes":      {ServiceType: "echo"},
		"duplicate schemes":    {ServiceType: "echo", AcceptedSchemes: []string{"https", "https"}},
		"unknown scheme":       {ServiceType: "echo", AcceptedSchemes: []string{"waku"}},
	}
	unknown := &applicationv1.ResolveServiceRequest{ServiceType: "echo", AcceptedSchemes: []string{"https"}}
	unknown.ProtoReflect().SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 99, protowire.VarintType), 1))
	requests["unknown field"] = unknown

	for name, request := range requests {
		t.Run(name, func(t *testing.T) {
			_, err := client.Resolve(context.Background(), connect.NewRequest(request))
			require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
			require.Equal(t, applicationv1.ErrorCode_ERROR_CODE_INVALID_ARGUMENT, discoveryApplicationError(t, err).GetCode())
		})
	}
	require.Zero(t, locator.calls)
}

func TestDiscoveryHandlerPreservesStableTypedLocatorErrors(t *testing.T) {
	locator := &locatorStub{}
	client := newAdmittedDiscoveryClient(t, locator, []identityaccess.Action{applicationdiscovery.ActionResolve})
	tests := []struct {
		err       error
		connect   connect.Code
		protocol  applicationv1.ErrorCode
		retryable bool
	}{
		{applicationdiscovery.ErrNotFound, connect.CodeNotFound, applicationv1.ErrorCode_ERROR_CODE_NOT_FOUND, false},
		{applicationdiscovery.ErrUnavailable, connect.CodeUnavailable, applicationv1.ErrorCode_ERROR_CODE_UNAVAILABLE, true},
		{errors.New("private invariant detail"), connect.CodeInternal, applicationv1.ErrorCode_ERROR_CODE_INTERNAL, false},
	}
	for _, test := range tests {
		locator.err = test.err
		_, err := client.Resolve(context.Background(), connect.NewRequest(&applicationv1.ResolveServiceRequest{
			ServiceType: "echo", AcceptedSchemes: []string{"https"},
		}))
		require.Equal(t, test.connect, connect.CodeOf(err))
		detail := discoveryApplicationError(t, err)
		require.Equal(t, test.protocol, detail.GetCode())
		require.Equal(t, applicationdiscovery.ActionResolve, detail.GetOperation())
		require.Equal(t, test.retryable, detail.GetRetryable())
		require.NotContains(t, detail.GetMessage(), "private invariant")
	}
}

func newAdmittedDiscoveryClient(t *testing.T, locator applicationdiscovery.ServiceLocator, actions []identityaccess.Action) applicationv1connect.DiscoveryServiceClient {
	return newDiscoveryClient(t, locator, actions, true)
}

func newDiscoveryClient(t *testing.T, locator applicationdiscovery.ServiceLocator, actions []identityaccess.Action, withSession bool) applicationv1connect.DiscoveryServiceClient {
	t.Helper()
	fixture := testkit.NewApplicationPrincipalAccess(t, actions)
	injector, extractor := applicationcall.NewChannel()
	contracts, registrations, err := applicationdiscovery.ProtectedProcedureSet()
	require.NoError(t, err)
	registry, err := applicationadmission.NewRegistry(contracts, registrations)
	require.NoError(t, err)
	interceptor, err := applicationadmission.NewInterceptor(applicationadmission.Config{
		Access: fixture.Service, Node: fixture.Node, FallbackPeer: fixture.Peer,
		FallbackSource: fixture.Source, Injector: injector, Registry: registry,
	})
	require.NoError(t, err)
	path, handler, err := applicationdiscovery.NewHTTPHandler(locator, extractor, interceptor)
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	options := []connect.ClientOption{
		connect.WithReadMaxBytes(applicationv1.MaxUnaryMessageBytes),
		connect.WithSendMaxBytes(applicationv1.MaxUnaryMessageBytes),
	}
	if withSession {
		authorization := "ArdentsApplicationSession " + base64.RawURLEncoding.EncodeToString(fixture.Session[:])
		options = append(options, connect.WithInterceptors(discoverySessionHeader{authorization: authorization}))
	}
	return applicationv1connect.NewDiscoveryServiceClient(server.Client(), server.URL, options...)
}

type locatorStub struct {
	targets []applicationdiscovery.Target
	err     error
	calls   int
}

func (l *locatorStub) Resolve(applicationdiscovery.Query) ([]applicationdiscovery.Target, error) {
	l.calls++
	return append([]applicationdiscovery.Target(nil), l.targets...), l.err
}

type discoverySessionHeader struct{ authorization string }

func (i discoverySessionHeader) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		request.Header().Set("Authorization", i.authorization)
		return next(ctx, request)
	}
}
func (discoverySessionHeader) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}
func (discoverySessionHeader) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func discoveryApplicationError(t *testing.T, err error) *applicationv1.ApplicationError {
	t.Helper()
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	for _, detail := range connectErr.Details() {
		value, detailErr := detail.Value()
		require.NoError(t, detailErr)
		if applicationErr, ok := value.(*applicationv1.ApplicationError); ok {
			return applicationErr
		}
	}
	require.FailNow(t, "connect error has no ApplicationError detail")
	return nil
}
