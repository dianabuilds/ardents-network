//go:build e2e

package localapi_e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	appdata "ardents/internal/content"
	runtimeinfra "ardents/internal/daemon"
	discoveryapi "ardents/internal/discovery"
	ardentsv1 "ardents/internal/localapi/protocol"
	transport "ardents/internal/network"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestSurfaceReadyHelper(t *testing.T) {
	if os.Getenv("ARDENTS_SURFACE_READY_HELPER") != "1" {
		return
	}
	generation := os.Getenv("ARDENTS_WORKLOAD_GENERATION")
	server := &http.Server{Addr: os.Getenv("ARDENTS_SURFACE_READY_ADDRESS"), Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Ardents-Generation", generation)
		w.WriteHeader(http.StatusNoContent)
	})}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		os.Exit(2)
	}
}

func TestOperatorNetworkSurfaceLifecycle(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerE2E,
		Domain:      "local-control-surface",
		ScenarioID:  "E2E-NET-SURFACE-001",
		Suite:       "e2e",
		Tags:        []string{"integration", "e2e", "local-control-surface", "network"},
		Speed:       "default",
		Environment: "local",
	})

	testkit.ConfigureLoopbackTransport(t)
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	remotePort := reserveSurfacePort(t)
	remote := testkit.StartNode(t, runtimeinfra.Config{
		Name: "operator-network-remote", NodeProfile: transport.NodeProfileServiceNode,
		Boot:      runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Transport: runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: transport.ReachabilityPrivateLAN},
		Data:      runtimeinfra.DataConfig{Dir: t.TempDir()}, Privacy: privacy.Sender, DiscoveryRefreshInterval: 50 * time.Millisecond,
		Workload: []runtimeinfra.WorkloadConfig{{
			ID:      "work.remote.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  surfaceReadyConfig(t, remotePort),
			Desired: "running",
			Services: []runtimeinfra.ServiceConfig{{
				ID:             "svc.remote.echo",
				Type:           "echo",
				Mode:           "NetworkPublished",
				Endpoints:      []string{surfaceAdvertisedEndpoint(t, remotePort)},
				ProbeEndpoints: []string{fmt.Sprintf("http://127.0.0.1:%d/ready", remotePort)},
			}},
		}},
	})
	testkit.WaitForServiceMatchCount(t, 10*time.Second, remote, "echo", 1)

	rt := testkit.NewRuntime(t, runtimeinfra.Config{
		Name:        "operator-network-sut",
		NodeProfile: transport.NodeProfileServiceNode,
		Boot:        runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Trust:       runtimeinfra.TrustConfig{Anchors: []string{remote.Snapshot().Ident.PublicKey}},
		Transport:   runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: transport.ReachabilityPrivateLAN},
		Data:        runtimeinfra.DataConfig{Dir: t.TempDir()}, Privacy: privacy.Receiver,
	})
	t.Cleanup(func() { _ = rt.Runtime.Stop(context.Background()) })
	client := testkit.NewArdentsClient(t, rt.Runtime)

	scenario.Precondition("start node through public control surface", func(t *testing.T) {
		require.NoError(t, rt.Runtime.Start(context.Background()))
	})

	scenario.Step("operator reads node, network and discovery readiness", func(t *testing.T) {
		status, err := client.GetNodeStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetNodeStatusRequest{}))
		require.NoError(t, err)
		require.Equal(t, "ready", status.Msg.GetSnapshot().GetNode().GetState())

		network, err := client.GetNetworkStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetNetworkStatusRequest{}))
		require.NoError(t, err)
		require.Equal(t, "ready", network.Msg.GetNetwork().GetState())
		require.Equal(t, "service_node", network.Msg.GetNetwork().GetNodeProfile())
		require.Equal(t, "ardents-private/1", network.Msg.GetNetwork().GetPrivacyProfile())
		require.Equal(t, "active", network.Msg.GetNetwork().GetPrivacyState())
		require.Equal(t, "capability_ready", network.Msg.GetNetwork().GetPrivacySwitchReason())
		require.Equal(t, "steady", network.Msg.GetNetwork().GetPrivacyRecoveryState())
		require.Empty(t, network.Msg.GetNetwork().GetPrivacyErrorCategories())

		discovery, err := client.GetDiscoveryStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetDiscoveryStatusRequest{}))
		require.NoError(t, err)
		require.Equal(t, "ready", discovery.Msg.GetDiscovery().GetState())
		require.GreaterOrEqual(t, discovery.Msg.GetDiscovery().GetLocalRecords(), int32(1))
	})

	scenario.Step("operator subscribes to node events and sees route absence before discovery input", func(t *testing.T) {
		before, err := client.ListRouteCandidates(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ListRouteCandidatesRequest{Service: "echo"}))
		require.NoError(t, err)
		require.Empty(t, before.Msg.GetCandidates())
		require.Equal(t, "not_found", before.Msg.GetRoute().GetOutcome())

		records, err := remote.ListRecords()
		require.NoError(t, err)
		require.NotEmpty(t, records)
		for _, record := range records {
			record.Source = "bootstrap"
			_, err := client.ImportRecord(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ImportRecordRequest{
				Record: toProtoDiscoveryRecord(record),
			}))
			require.NoError(t, err)
		}

		testkit.WaitForCondition(t, 10*time.Second, "discovery truth and route candidates become visible", func() (bool, string) {
			discovery, err := client.GetDiscoveryStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetDiscoveryStatusRequest{}))
			if err != nil {
				return false, err.Error()
			}
			routes, err := client.ListRouteCandidates(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ListRouteCandidatesRequest{Service: "echo"}))
			if err != nil {
				return false, err.Error()
			}
			if discovery.Msg.GetDiscovery().GetRemoteRecords() == 0 {
				return false, "remote records are not visible yet"
			}
			if len(routes.Msg.GetCandidates()) == 0 {
				return false, "route candidates are not visible yet"
			}
			return true, ""
		})

		resolved, err := client.ResolveService(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ResolveServiceRequest{Service: "echo"}))
		require.NoError(t, err)
		require.NotEmpty(t, resolved.Msg.GetMatches())

		recent, err := client.ListRecentEvents(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ListRecentEventsRequest{Limit: 20}))
		require.NoError(t, err)
		require.Contains(t, recentEventsSummary(recent.Msg.GetEvents()), "discovery/imported")
	})

	scenario.Assert("operator sees shutdown truth through public surface", func(t *testing.T) {
		require.NoError(t, rt.Runtime.Stop(context.Background()))

		status, err := client.GetNodeStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetNodeStatusRequest{}))
		require.NoError(t, err)
		require.Equal(t, "stopped", status.Msg.GetSnapshot().GetNode().GetState())

		network, err := client.GetNetworkStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetNetworkStatusRequest{}))
		require.NoError(t, err)
		require.Equal(t, "stopped", network.Msg.GetNetwork().GetState())
	})
}

func TestOperatorServiceAndDataSurfaceReadiness(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerE2E,
		Domain:      "local-control-surface",
		ScenarioID:  "E2E-SVC-DATA-SURFACE-001",
		Suite:       "e2e",
		Tags:        []string{"integration", "e2e", "local-control-surface", "workload", "data"},
		Speed:       "default",
		Environment: "local",
	})

	testkit.ConfigureLoopbackTransport(t)
	sourceDir := t.TempDir()
	sourceStore := appdata.NewInDir(sourceDir)
	require.NoError(t, sourceStore.Load())
	key := []byte("0123456789abcdef0123456789abcdef")
	stored, err := sourceStore.StoreEncryptedBlob(appdata.Blob{MediaType: "application/octet-stream"}, []byte("network payload"), key, "")
	require.NoError(t, err)
	discoveryPrivacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))

	source := testkit.StartNode(t, runtimeinfra.Config{
		Name: "operator-svc-data-source",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: sourceDir}, Privacy: discoveryPrivacy.Sender,
	})
	records, err := source.ListRecords()
	require.NoError(t, err)
	require.NotEmpty(t, records)
	require.NotNil(t, records[0].Node)

	rt := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "operator-svc-data-sut", NodeProfile: transport.NodeProfileServiceNode,
		Boot:      runtimeinfra.BootConfig{Sources: append([]string(nil), records[0].Node.Endpoints...)},
		Trust:     runtimeinfra.TrustConfig{Anchors: []string{source.Snapshot().Ident.PublicKey}},
		Transport: runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: transport.ReachabilityPrivateLAN},
		Data:      runtimeinfra.DataConfig{Dir: t.TempDir()}, Privacy: discoveryPrivacy.Receiver,
		DiscoveryRefreshInterval: 500 * time.Millisecond,
	})
	t.Cleanup(func() { _ = rt.Runtime.Stop(context.Background()) })
	client := testkit.NewArdentsClient(t, rt.Runtime)

	scenario.Precondition("start runtime and register workload through public control surface", func(t *testing.T) {
		require.NoError(t, rt.Runtime.Start(context.Background()))
		port := reserveSurfacePort(t)

		_, err := client.RegisterWorkload(context.Background(), testkit.AuthorizedRequest(&ardentsv1.RegisterWorkloadRequest{
			Spec: &ardentsv1.WorkloadSpecSnapshot{
				Id:      "work.echo",
				Kind:    "service",
				Owner:   "node",
				Config:  surfaceReadyConfig(t, port),
				Desired: "present",
				Services: []*ardentsv1.PublishedServiceSnapshot{{
					Id:             "svc.work.echo",
					Type:           "echo",
					Owner:          "node",
					Mode:           "NetworkPublished",
					Endpoints:      []string{surfaceAdvertisedEndpoint(t, port)},
					ProbeEndpoints: []string{fmt.Sprintf("http://127.0.0.1:%d/ready", port)},
				}},
			},
		}))
		require.NoError(t, err)
	})

	scenario.Step("operator starts workload and observes publication truth", func(t *testing.T) {
		require.NoError(t, rt.Workload.Start(context.Background(), "work.echo"))

		testkit.WaitForCondition(t, 10*time.Second, "hosted service becomes published", func() (bool, string) {
			hosted, err := client.GetHostedService(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetHostedServiceRequest{Id: "svc.work.echo"}))
			if err != nil {
				return false, err.Error()
			}
			if !hosted.Msg.GetService().GetPublished() {
				return false, hosted.Msg.GetService().GetState()
			}
			publication, err := client.GetServicePublicationStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetServicePublicationStatusRequest{Id: "svc.work.echo"}))
			if err != nil {
				return false, err.Error()
			}
			if !publication.Msg.GetPublication().GetPublished() {
				return false, publication.Msg.GetPublication().GetState()
			}
			return true, ""
		})

		list, err := client.ListHostedServices(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ListHostedServicesRequest{}))
		require.NoError(t, err)
		require.NotEmpty(t, list.Msg.GetServices())
	})

	scenario.Step("operator executes data fetch flow and observes transfer truth", func(t *testing.T) {
		fetchCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		fetched, err := rt.Runtime.FetchBlob(fetchCtx, stored.ID)
		require.NoError(t, err)
		require.Equal(t, stored.ID, fetched.ID)

		testkit.WaitForCondition(t, 10*time.Second, "transfer and local source become visible", func() (bool, string) {
			transfers, err := client.ListTransfers(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ListTransfersRequest{}))
			if err != nil {
				return false, err.Error()
			}
			for _, item := range transfers.Msg.GetTransfers() {
				if item.GetResourceId() == stored.ID && item.GetState() == "completed" {
					sources, err := client.ListBlobSources(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ListBlobSourcesRequest{Id: stored.ID}))
					if err != nil {
						return false, err.Error()
					}
					if len(sources.Msg.GetSources()) == 0 {
						return false, "blob sources are not visible yet"
					}
					return true, ""
				}
			}
			return false, "completed transfer is not visible yet"
		})

		transfers, err := client.ListTransfers(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ListTransfersRequest{}))
		require.NoError(t, err)
		transfer := findTransferByResource(t, transfers.Msg.GetTransfers(), stored.ID)

		getTransfer, err := client.GetTransfer(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetTransferRequest{Id: transfer.GetId()}))
		require.NoError(t, err)
		require.Equal(t, "completed", getTransfer.Msg.GetTransfer().GetState())

		health, err := client.GetHealthSummary(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetHealthSummaryRequest{}))
		require.NoError(t, err)
		require.Equalf(t, "ready", health.Msg.GetHealth().GetState(), "health=%s", health.Msg.GetHealth().String())
	})

	scenario.Degraded("operator sees publication withdrawal and explanation after workload stop", func(t *testing.T) {
		require.NoError(t, rt.Workload.Stop(context.Background(), "work.echo"))

		testkit.WaitForCondition(t, 10*time.Second, "publication is withdrawn after workload stop", func() (bool, string) {
			hosted, err := client.GetHostedService(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetHostedServiceRequest{Id: "svc.work.echo"}))
			if err != nil {
				return false, err.Error()
			}
			if hosted.Msg.GetService().GetPublished() {
				return false, hosted.Msg.GetService().GetState()
			}
			return true, ""
		})

		hosted, err := client.GetHostedService(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetHostedServiceRequest{Id: "svc.work.echo"}))
		require.NoError(t, err)
		require.False(t, hosted.Msg.GetService().GetPublished())
		require.Equal(t, "inactive", hosted.Msg.GetService().GetState())

		publication, err := client.GetServicePublicationStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetServicePublicationStatusRequest{Id: "svc.work.echo"}))
		require.NoError(t, err)
		require.False(t, publication.Msg.GetPublication().GetPublished())

		explanation, err := client.ExplainFailure(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ExplainFailureRequest{
			Scope:      "service",
			ResourceId: "svc.work.echo",
		}))
		require.NoError(t, err)
		require.NotNil(t, explanation.Msg.GetExplanation().GetReason())
		require.Equal(t, "service.publication.unavailable", explanation.Msg.GetExplanation().GetReason().GetCode())
	})
}

func surfaceReadyConfig(t *testing.T, port int) string {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	raw, err := json.Marshal(map[string]any{"command": executable, "args": []string{"-test.run=TestSurfaceReadyHelper"},
		"env": map[string]string{"ARDENTS_SURFACE_READY_HELPER": "1", "ARDENTS_SURFACE_READY_ADDRESS": fmt.Sprintf("0.0.0.0:%d", port)}})
	require.NoError(t, err)
	return string(raw)
}

func reserveSurfacePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { require.NoError(t, listener.Close()) }()
	return listener.Addr().(*net.TCPAddr).Port
}

//goland:noinspection ALL
func surfaceAdvertisedEndpoint(t *testing.T, port int) string {
	t.Helper()
	addresses, err := net.InterfaceAddrs()
	require.NoError(t, err)
	for _, address := range addresses {
		ip, _, parseErr := net.ParseCIDR(address.String())
		if parseErr == nil && ip.To4() != nil && ip.IsPrivate() && !ip.IsLoopback() {
			return fmt.Sprintf("http://%s:%d/ready", ip.String(), port)
		}
	}
	t.Fatal("Linux test container has no private IPv4 address")
	return ""
}

func toProtoDiscoveryRecord(record discoveryapi.CatalogRecordSnapshot) *ardentsv1.DiscoveryRecord {
	out := &ardentsv1.DiscoveryRecord{
		Version:     record.Version,
		IssuedAtV1:  timestampProto(record.IssuedAt),
		ExpiresAtV1: timestampProto(record.ExpiresAt),
		SignatureV1: record.Signature,
		SourceV1:    record.Source,
	}
	if record.Node != nil {
		out.Facts = &ardentsv1.DiscoveryRecord_NodeFacts{NodeFacts: &ardentsv1.NodeDiscoveryFacts{
			Principal: record.Node.Principal,
			PublicKey: record.Node.PublicKey,
			Endpoints: append([]string(nil), record.Node.Endpoints...),
		}}
	}
	if record.Service != nil {
		out.Facts = &ardentsv1.DiscoveryRecord_ServiceFacts{ServiceFacts: &ardentsv1.ServiceDiscoveryFacts{
			ServiceId:     record.Service.ID,
			ServiceType:   record.Service.Type,
			NodePrincipal: record.Service.NodePrincipal,
			WorkloadId:    record.Service.WorkloadID,
			Mode:          record.Service.Mode,
			PublicKey:     record.Service.PublicKey,
			Endpoints:     append([]string(nil), record.Service.Endpoints...),
		}}
	}
	return out
}

func timestampProto(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

func findTransferByResource(t *testing.T, transfers []*ardentsv1.TransferSnapshot, resourceID string) *ardentsv1.TransferSnapshot {
	t.Helper()
	for _, item := range transfers {
		if item.GetResourceId() == resourceID {
			return item
		}
	}
	require.FailNow(t, "expected transfer for resource", "resource_id=%s", resourceID)
	return nil
}

func recentEventsSummary(events []*ardentsv1.EventEnvelope) []string {
	out := make([]string, 0, len(events))
	for _, item := range events {
		out = append(out, item.GetDomain()+"/"+item.GetType())
	}
	return out
}
