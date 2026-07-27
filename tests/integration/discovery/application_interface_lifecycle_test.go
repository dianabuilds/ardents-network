//go:build integration

package discovery_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	applicationadmission "ardents/internal/applicationapi/admission"
	applicationcall "ardents/internal/applicationapi/call"
	applicationdiscovery "ardents/internal/applicationapi/discovery"
	runtimeconfig "ardents/internal/config"
	runtimeinfra "ardents/internal/daemon"
	discoverytruth "ardents/internal/discovery"
	discoveryrecord "ardents/internal/discovery/records"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"
	routepolicy "ardents/internal/policy"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"
	"ardents/tests/testkit"
)

func TestApplicationDiscoveryResolvesTrustedImportedRecord(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "application-discovery",
		ScenarioID:  "APP-DISC-001",
		Suite:       "integration",
		Tags:        []string{"integration", "application-interface", "application-discovery", "lifecycle", "security"},
		Speed:       "default",
		Environment: "local",
	})
	fixture := newApplicationDiscoveryLifecycleFixture(t)

	response, err := fixture.client.Resolve(context.Background(), connect.NewRequest(
		&applicationv1.ResolveServiceRequest{
			ServiceType:     "echo",
			AcceptedSchemes: []string{"https"},
		},
	))

	require.NoError(t, err)
	require.Equal(t, []*applicationv1.ResolvedServiceTarget{{
		ServiceId: "svc.remote.echo",
		Endpoint:  "https://10.20.30.40:8443",
		Scheme:    "https",
	}}, response.Msg.GetTargets())
}

func TestApplicationDiscoveryAppliesImportedWithdrawalOnNextResolve(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "application-discovery",
		ScenarioID:  "APP-DISC-001",
		Suite:       "integration",
		Tags:        []string{"integration", "application-interface", "application-discovery", "lifecycle", "security"},
		Speed:       "default",
		Environment: "local",
	})
	fixture := newApplicationDiscoveryLifecycleFixture(t)
	request := connect.NewRequest(&applicationv1.ResolveServiceRequest{
		ServiceType: "echo", AcceptedSchemes: []string{"https"},
	})
	_, err := fixture.client.Resolve(context.Background(), request)
	require.NoError(t, err)

	withdrawn := fixture.record.Clone()
	withdrawn.Service.Endpoints = nil
	withdrawn.IssuedAt = fixture.now
	withdrawn.ExpiresAt = fixture.now.Add(time.Hour)
	signApplicationDiscoveryRecord(t, &withdrawn, fixture.publisher)
	imported, err := fixture.store.Import(withdrawn, discoveryrecord.Imported)
	require.NoError(t, err)
	require.True(t, imported.Applied)

	_, err = fixture.client.Resolve(context.Background(), connect.NewRequest(
		&applicationv1.ResolveServiceRequest{
			ServiceType: "echo", AcceptedSchemes: []string{"https"},
		},
	))

	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	require.Equal(t, applicationv1.ErrorCode_ERROR_CODE_NOT_FOUND, discoveryApplicationError(t, err).GetCode())
}

func TestApplicationDiscoveryAppliesTrustReloadOnNextResolve(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "application-discovery",
		ScenarioID:  "APP-DISC-001",
		Suite:       "integration",
		Tags:        []string{"integration", "application-interface", "application-discovery", "lifecycle", "security"},
		Speed:       "default",
		Environment: "local",
	})
	now := time.Now().UTC().Truncate(time.Second)
	record, publisher := signedImportedServiceRecord(t, now.Add(-time.Second), []string{
		"https://10.20.30.40:8443",
	})
	trustedRegistry, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: record.Service.NodePrincipal.String(),
		PublicKey: publisher.Public().(ed25519.PublicKey),
		Purposes:  []identitytrust.Purpose{identitytrust.PurposeDiscoveryPublish},
	}})
	require.NoError(t, err)
	doc := runtimeconfig.Defaults()
	trustedPrincipal := runtimeconfig.TrustedPrincipalConfig{
		Principal: record.Service.NodePrincipal.String(),
		PublicKey: base64.StdEncoding.EncodeToString(publisher.Public().(ed25519.PublicKey)),
		Purposes:  []identitytrust.Purpose{identitytrust.PurposeDiscoveryPublish},
	}
	doc.Trust.Principals = []runtimeconfig.TrustedPrincipalConfig{trustedPrincipal}
	configPath := filepath.Join(t.TempDir(), "ardents.json")
	writeApplicationDiscoveryOperatorDocument(t, configPath, doc)
	manager, err := runtimeconfig.NewManager(configPath, doc)
	require.NoError(t, err)
	node := runtimeinfra.NewNode(runtimeinfra.Config{
		Name:           "application-discovery-trust-reload",
		Data:           runtimeinfra.DataConfig{Dir: t.TempDir()},
		Trust:          runtimeinfra.TrustConfig{Registry: trustedRegistry},
		OperatorConfig: manager,
	})
	owners, ok := runtimeinfra.OwnersFor(node)
	require.True(t, ok)
	imported, err := owners.Discovery.Import(record, discoveryrecord.Imported)
	require.NoError(t, err)
	require.True(t, imported.Applied)
	truth, err := applicationdiscovery.NewMaintainedTruth(
		owners.Discovery, owners.DiscoveryTrust, owners.RoutePolicy,
	)
	require.NoError(t, err)
	locator, err := applicationdiscovery.NewLocator(truth)
	require.NoError(t, err)
	client := newDiscoveryClient(t, locator, []identityaccess.Action{applicationdiscovery.ActionResolve}, true)
	resolve := func() (*connect.Response[applicationv1.ResolveServiceResponse], error) {
		return client.Resolve(context.Background(), connect.NewRequest(
			&applicationv1.ResolveServiceRequest{
				ServiceType: "echo", AcceptedSchemes: []string{"https"},
			},
		))
	}
	_, err = resolve()
	require.NoError(t, err)

	for index := 0; index < 63; index++ {
		pressure := record.Clone()
		pressure.Service.ID = discoveryrecord.ServiceID(fmt.Sprintf("svc.revoked.%02d", index))
		pressure.Service.Workload = discoveryrecord.WorkloadID(fmt.Sprintf("work.revoked.%02d", index))
		pressure.Service.Type = "noise"
		signApplicationDiscoveryRecord(t, &pressure, publisher)
		result, importErr := owners.Discovery.Import(pressure, discoveryrecord.Bootstrap)
		require.NoError(t, importErr)
		require.True(t, result.Applied)
	}
	overflow := record.Clone()
	overflow.Service.ID = "svc.revoked.overflow"
	overflow.Service.Workload = "work.revoked.overflow"
	overflow.Service.Type = "noise"
	signApplicationDiscoveryRecord(t, &overflow, publisher)
	result, importErr := owners.Discovery.Import(overflow, discoveryrecord.Bootstrap)
	require.NoError(t, importErr)
	require.False(t, result.Applied)
	require.Equal(t, "rejected_capacity", result.Outcome)
	response, err := resolve()
	require.NoError(t, err)
	require.Len(t, response.Msg.GetTargets(), 1)

	doc.Trust.Principals = nil
	writeApplicationDiscoveryOperatorDocument(t, configPath, doc)
	reload := node.ReloadConfig(context.Background())
	require.Equal(t, runtimeconfig.OutcomeApplied, reload.Outcome)
	_, err = resolve()
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	require.Equal(t, applicationv1.ErrorCode_ERROR_CODE_NOT_FOUND, discoveryApplicationError(t, err).GetCode())

	doc.Trust.Principals = []runtimeconfig.TrustedPrincipalConfig{trustedPrincipal}
	writeApplicationDiscoveryOperatorDocument(t, configPath, doc)
	reload = node.ReloadConfig(context.Background())
	require.Equal(t, runtimeconfig.OutcomeApplied, reload.Outcome)
	response, err = resolve()
	require.NoError(t, err)
	require.Len(t, response.Msg.GetTargets(), 1)
}

func writeApplicationDiscoveryOperatorDocument(
	t *testing.T,
	path string,
	doc runtimeconfig.Document,
) {
	t.Helper()
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))
}

func TestApplicationDiscoveryAppliesRoutePolicyReloadOnNextResolve(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "application-discovery",
		ScenarioID:  "APP-DISC-001",
		Suite:       "integration",
		Tags:        []string{"integration", "application-interface", "application-discovery", "lifecycle", "security"},
		Speed:       "default",
		Environment: "local",
	})
	now := time.Now().UTC().Truncate(time.Second)
	record, publisher := signedImportedServiceRecord(t, now.Add(-time.Second), []string{
		"https://10.20.30.40:8443",
	})
	trustedRegistry, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: record.Service.NodePrincipal.String(),
		PublicKey: publisher.Public().(ed25519.PublicKey),
		Purposes:  []identitytrust.Purpose{identitytrust.PurposeDiscoveryPublish},
	}})
	require.NoError(t, err)
	doc := runtimeconfig.Defaults()
	doc.Trust.Principals = []runtimeconfig.TrustedPrincipalConfig{{
		Principal: record.Service.NodePrincipal.String(),
		PublicKey: base64.StdEncoding.EncodeToString(publisher.Public().(ed25519.PublicKey)),
		Purposes:  []identitytrust.Purpose{identitytrust.PurposeDiscoveryPublish},
	}}
	configPath := filepath.Join(t.TempDir(), "ardents.json")
	writeApplicationDiscoveryOperatorDocument(t, configPath, doc)
	manager, err := runtimeconfig.NewManager(configPath, doc)
	require.NoError(t, err)
	node := runtimeinfra.NewNode(runtimeinfra.Config{
		Name:           "application-discovery-policy-reload",
		Data:           runtimeinfra.DataConfig{Dir: t.TempDir()},
		Trust:          runtimeinfra.TrustConfig{Registry: trustedRegistry},
		OperatorConfig: manager,
	})
	owners, ok := runtimeinfra.OwnersFor(node)
	require.True(t, ok)
	imported, err := owners.Discovery.Import(record, discoveryrecord.Imported)
	require.NoError(t, err)
	require.True(t, imported.Applied)
	truth, err := applicationdiscovery.NewMaintainedTruth(
		owners.Discovery, owners.DiscoveryTrust, owners.RoutePolicy,
	)
	require.NoError(t, err)
	locator, err := applicationdiscovery.NewLocator(truth)
	require.NoError(t, err)
	client := newDiscoveryClient(t, locator, []identityaccess.Action{applicationdiscovery.ActionResolve}, true)
	resolve := func() (*connect.Response[applicationv1.ResolveServiceResponse], error) {
		return client.Resolve(context.Background(), connect.NewRequest(
			&applicationv1.ResolveServiceRequest{
				ServiceType: "echo", AcceptedSchemes: []string{"https"},
			},
		))
	}
	_, err = resolve()
	require.NoError(t, err)

	doc.Policy.DeniedRouteSchemes = []string{"https"}
	writeApplicationDiscoveryOperatorDocument(t, configPath, doc)
	reload := node.ReloadConfig(context.Background())
	require.Equal(t, runtimeconfig.OutcomeApplied, reload.Outcome)
	_, err = resolve()
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	require.Equal(t, applicationv1.ErrorCode_ERROR_CODE_NOT_FOUND, discoveryApplicationError(t, err).GetCode())

	doc.Policy.DeniedRouteSchemes = nil
	writeApplicationDiscoveryOperatorDocument(t, configPath, doc)
	reload = node.ReloadConfig(context.Background())
	require.Equal(t, runtimeconfig.OutcomeApplied, reload.Outcome)
	response, err := resolve()
	require.NoError(t, err)
	require.Len(t, response.Msg.GetTargets(), 1)
}

func TestApplicationDiscoveryRejectsUntrustedBootstrapPressureBeforeProjection(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "application-discovery",
		ScenarioID:  "APP-DISC-001",
		Suite:       "integration",
		Tags:        []string{"integration", "application-interface", "application-discovery", "lifecycle", "security"},
		Speed:       "default",
		Environment: "local",
	})
	fixture := newApplicationDiscoveryLifecycleFixture(t)
	for index := 0; index <= 64; index++ {
		record, private := signedImportedServiceRecordWithID(
			t,
			fmt.Sprintf("svc.untrusted.%02d", index),
			fixture.now.Add(-time.Second),
			[]string{"https://10.20.30.41:8443"},
		)
		result, err := fixture.store.Import(record, discoveryrecord.Bootstrap)
		clear(private)
		require.NoError(t, err)
		require.False(t, result.Applied)
		require.Equal(t, "rejected_untrusted", result.Outcome)
	}

	response, err := fixture.client.Resolve(context.Background(), connect.NewRequest(
		&applicationv1.ResolveServiceRequest{
			ServiceType: "echo", AcceptedSchemes: []string{"https"},
		},
	))

	require.NoError(t, err)
	require.Equal(t, "svc.remote.echo", response.Msg.GetTargets()[0].GetServiceId())
}

type applicationDiscoveryLifecycleFixture struct {
	now       time.Time
	record    discoverytruth.Record
	publisher ed25519.PrivateKey
	store     *discoverytruth.Service
	policy    *routepolicy.Service
	client    applicationv1connect.DiscoveryServiceClient
}

func newApplicationDiscoveryLifecycleFixture(t *testing.T) applicationDiscoveryLifecycleFixture {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	record, publisher := signedImportedServiceRecord(t, now.Add(-time.Second), []string{
		"https://10.20.30.40:8443",
	})
	trustRegistry, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: record.Service.NodePrincipal.String(),
		PublicKey: publisher.Public().(ed25519.PublicKey),
		Purposes:  []identitytrust.Purpose{identitytrust.PurposeDiscoveryPublish},
	}})
	require.NoError(t, err)
	trust := discoverytruth.NewTrustEvaluator(trustRegistry)
	store := discoverytruth.NewInDirWithTrust(t.TempDir(), trust)
	imported, err := store.Import(record, discoveryrecord.Imported)
	require.NoError(t, err)
	require.True(t, imported.Applied)
	policy := routepolicy.New(routepolicy.Config{})
	truth, err := applicationdiscovery.NewMaintainedTruth(store, trust, policy)
	require.NoError(t, err)
	locator, err := applicationdiscovery.NewLocator(truth)
	require.NoError(t, err)
	return applicationDiscoveryLifecycleFixture{
		now: now, record: record, publisher: publisher, store: store, policy: policy,
		client: newDiscoveryClient(
			t, locator, []identityaccess.Action{applicationdiscovery.ActionResolve}, true,
		),
	}
}

func signedImportedServiceRecord(
	t *testing.T,
	now time.Time,
	endpoints []string,
) (discoverytruth.Record, ed25519.PrivateKey) {
	return signedImportedServiceRecordWithID(t, "svc.remote.echo", now, endpoints)
}

func signedImportedServiceRecordWithID(
	t *testing.T,
	serviceID string,
	now time.Time,
	endpoints []string,
) (discoverytruth.Record, ed25519.PrivateKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	node, err := identityprincipal.FromEd25519PublicKey(public)
	require.NoError(t, err)
	record := discoverytruth.Record{
		Version: discoveryrecord.Version,
		Service: &discoveryrecord.ServiceFacts{
			ID:            discoveryrecord.ServiceID(serviceID),
			Type:          "echo",
			NodePrincipal: node,
			Workload:      "work.remote.echo",
			Mode:          "NetworkPublished",
			PublicKey:     base64.StdEncoding.EncodeToString(public),
			Endpoints:     append([]string(nil), endpoints...),
		},
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour),
	}
	payload, err := discoverytruth.Canonical(record)
	require.NoError(t, err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
	return record, private
}

func signApplicationDiscoveryRecord(t *testing.T, record *discoverytruth.Record, private ed25519.PrivateKey) {
	t.Helper()
	payload, err := discoverytruth.Canonical(*record)
	require.NoError(t, err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
}

func newDiscoveryClient(
	t *testing.T,
	locator applicationdiscovery.ServiceLocator,
	actions []identityaccess.Action,
	withSession bool,
) applicationv1connect.DiscoveryServiceClient {
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
		options = append(options, connect.WithInterceptors(applicationDiscoverySessionHeader{authorization: authorization}))
	}
	return applicationv1connect.NewDiscoveryServiceClient(server.Client(), server.URL, options...)
}

type applicationDiscoverySessionHeader struct {
	authorization string
}

func (i applicationDiscoverySessionHeader) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		request.Header().Set("Authorization", i.authorization)
		return next(ctx, request)
	}
}

func (applicationDiscoverySessionHeader) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (applicationDiscoverySessionHeader) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func discoveryApplicationError(t *testing.T, err error) *applicationv1.ApplicationError {
	t.Helper()
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	for _, detail := range connectErr.Details() {
		value, detailErr := detail.Value()
		require.NoError(t, detailErr)
		if applicationError, ok := value.(*applicationv1.ApplicationError); ok {
			return applicationError
		}
	}
	t.Fatal("application error detail is absent")
	return nil
}
