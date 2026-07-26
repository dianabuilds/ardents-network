package discovery_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
	truth, err := applicationdiscovery.NewMaintainedTruth(store, trust, allowRoutePolicy{})
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
		"non-ASCII service type": {
			ServiceType: "écho", AcceptedSchemes: []string{"https"},
		},
		"oversized service type": {
			ServiceType: strings.Repeat("a", 513), AcceptedSchemes: []string{"https"},
		},
		"missing schemes":   {ServiceType: "echo"},
		"duplicate schemes": {ServiceType: "echo", AcceptedSchemes: []string{"https", "https"}},
		"too many schemes":  {ServiceType: "echo", AcceptedSchemes: []string{"https", "http", "tcp", "https"}},
		"empty scheme":      {ServiceType: "echo", AcceptedSchemes: []string{""}},
		"uppercase scheme":  {ServiceType: "echo", AcceptedSchemes: []string{"HTTPS"}},
		"unknown scheme":    {ServiceType: "echo", AcceptedSchemes: []string{"waku"}},
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

func TestDiscoveryRejectsUnknownJSONFieldBeforeAdmissionOrLocator(t *testing.T) {
	locator := &locatorStub{}
	_, extractor := applicationcall.NewChannel()
	_, handler, err := applicationdiscovery.NewHTTPHandler(locator, extractor)
	require.NoError(t, err)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	request, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost,
		server.URL+applicationv1connect.DiscoveryServiceResolveProcedure,
		strings.NewReader(`{"serviceType":"echo","acceptedSchemes":["https"],"surprise":true}`),
	)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Connect-Protocol-Version", "1")

	response, err := server.Client().Do(request)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, response.Body.Close()) })

	require.Equal(t, http.StatusBadRequest, response.StatusCode)
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

func TestDiscoveryAuthorizedIneligibleOutcomesArePubliclyUniformNotFound(t *testing.T) {
	tests := []struct {
		name    string
		truth   *projectionTruth
		schemes []string
	}{
		{name: "absent", truth: &projectionTruth{}, schemes: []string{"https"}},
		{name: "expired", truth: &projectionTruth{
			entries: []discoverytruth.Entry{
				serviceEntry("svc.echo", "echo", "NetworkPublished", "https://10.20.30.40:8443"),
			},
			trust: discoverytruth.TrustResult{Outcome: "expired", Reason: "private expiry time"},
		}, schemes: []string{"https"}},
		{name: "withdrawn", truth: &projectionTruth{
			entries: []discoverytruth.Entry{
				serviceEntry("svc.echo", "echo", "NetworkPublished"),
			},
			trust: discoverytruth.TrustResult{Valid: true, Trusted: true, Usable: true},
		}, schemes: []string{"https"}},
		{name: "wrong mode", truth: &projectionTruth{
			entries: []discoverytruth.Entry{
				serviceEntry("svc.echo", "echo", "LocalOnly", "https://10.20.30.40:8443"),
			},
			trust: discoverytruth.TrustResult{Valid: true, Trusted: true, Usable: true},
		}, schemes: []string{"https"}},
		{name: "untrusted", truth: &projectionTruth{
			entries: []discoverytruth.Entry{
				serviceEntry("svc.echo", "echo", "NetworkPublished", "https://10.20.30.40:8443"),
			},
			trust: discoverytruth.TrustResult{Valid: true, Reason: "private trust reason"},
		}, schemes: []string{"https"}},
		{name: "unsafe endpoint", truth: &projectionTruth{
			entries: []discoverytruth.Entry{
				serviceEntry("svc.echo", "echo", "NetworkPublished", "https://127.0.0.1:8443"),
			},
		}, schemes: []string{"https"}},
		{name: "policy denied", truth: &projectionTruth{
			entries: []discoverytruth.Entry{
				serviceEntry("svc.echo", "echo", "NetworkPublished", "https://10.20.30.40:8443"),
			},
			policyErr: errors.New("private policy reason"),
		}, schemes: []string{"https"}},
		{name: "scheme mismatch", truth: &projectionTruth{
			entries: []discoverytruth.Entry{
				serviceEntry("svc.echo", "echo", "NetworkPublished", "http://10.20.30.40:8080"),
			},
		}, schemes: []string{"https"}},
	}

	want := discoveryPublicError{
		connectCode: connect.CodeNotFound,
		protocol:    applicationv1.ErrorCode_ERROR_CODE_NOT_FOUND,
		operation:   applicationdiscovery.ActionResolve,
		message:     "service was not found",
		retryable:   false,
		detailCount: 1,
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			locator, err := applicationdiscovery.NewLocator(test.truth)
			require.NoError(t, err)
			client := newAdmittedDiscoveryClient(t, locator, []identityaccess.Action{
				applicationdiscovery.ActionResolve,
			})

			_, err = client.Resolve(context.Background(), connect.NewRequest(&applicationv1.ResolveServiceRequest{
				ServiceType: "echo", AcceptedSchemes: test.schemes,
			}))

			require.Equal(t, want, publicDiscoveryError(t, err))
		})
	}
}

func TestDiscoveryHandlerRejectsLocatorInvariantViolations(t *testing.T) {
	tests := []struct {
		name    string
		targets []applicationdiscovery.Target
	}{
		{
			name: "unsafe endpoint",
			targets: []applicationdiscovery.Target{{
				ServiceID: "svc.echo", Endpoint: "https://127.0.0.1:8443", Scheme: "https",
			}},
		},
		{
			name: "scheme not accepted by caller",
			targets: []applicationdiscovery.Target{{
				ServiceID: "svc.echo", Endpoint: "tcp://10.20.30.40:9000", Scheme: "tcp",
			}},
		},
		{
			name: "duplicate pair",
			targets: []applicationdiscovery.Target{
				{ServiceID: "svc.echo", Endpoint: "https://10.20.30.40:8443", Scheme: "https"},
				{ServiceID: "svc.echo", Endpoint: "https://10.20.30.40:8443", Scheme: "https"},
			},
		},
		{
			name: "unsorted",
			targets: []applicationdiscovery.Target{
				{ServiceID: "svc.z", Endpoint: "https://10.20.30.40:8443", Scheme: "https"},
				{ServiceID: "svc.a", Endpoint: "https://10.20.30.40:8443", Scheme: "https"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newAdmittedDiscoveryClient(t, &locatorStub{targets: test.targets}, []identityaccess.Action{
				applicationdiscovery.ActionResolve,
			})

			_, err := client.Resolve(context.Background(), connect.NewRequest(&applicationv1.ResolveServiceRequest{
				ServiceType: "echo", AcceptedSchemes: []string{"https", "http"},
			}))

			require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
			detail := discoveryApplicationError(t, err)
			require.Equal(t, applicationv1.ErrorCode_ERROR_CODE_INTERNAL, detail.GetCode())
			require.Equal(t, "application discovery failed", detail.GetMessage())
		})
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

type discoveryPublicError struct {
	connectCode connect.Code
	protocol    applicationv1.ErrorCode
	operation   string
	message     string
	retryable   bool
	detailCount int
}

func publicDiscoveryError(t *testing.T, err error) discoveryPublicError {
	t.Helper()
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	detail := discoveryApplicationError(t, err)
	return discoveryPublicError{
		connectCode: connectErr.Code(),
		protocol:    detail.GetCode(),
		operation:   detail.GetOperation(),
		message:     detail.GetMessage(),
		retryable:   detail.GetRetryable(),
		detailCount: len(connectErr.Details()),
	}
}
