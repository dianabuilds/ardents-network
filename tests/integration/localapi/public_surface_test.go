//go:build integration

package localapi_test

import (
	"context"
	"testing"
	"time"

	appdata "ardents/internal/content"
	runtimeinfra "ardents/internal/daemon"
	discoveryapi "ardents/internal/discovery"
	"ardents/internal/hosting"
	ardentsv1 "ardents/internal/localapi/protocol"
	transport "ardents/internal/network"
	"ardents/internal/transfer"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestConnectRPCNetworkPublicSurfaceMatchesLocalTruth(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "local-control-surface",
		ScenarioID:  "INT-NET-SURFACE-001",
		Suite:       "integration",
		Tags:        []string{"integration", "local-control-surface", "network", "discovery"},
		Speed:       "default",
		Environment: "local",
	})

	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	config, endpoint, probe := localControlReadyFixture(t)
	remote := testkit.StartNode(t, runtimeinfra.Config{
		Name: "network-surface-remote", NodeProfile: transport.NodeProfileServiceNode,
		Boot:      runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Transport: runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: transport.ReachabilityPrivateLAN},
		Data:      runtimeinfra.DataConfig{Dir: t.TempDir()}, Privacy: privacy.Sender,
		Workload: []runtimeinfra.WorkloadConfig{{
			ID:      "work.remote.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  config,
			Desired: "running",
			Services: []runtimeinfra.ServiceConfig{{
				ID:             "svc.remote.echo",
				Type:           "echo",
				Mode:           "NetworkPublished",
				Endpoints:      []string{endpoint},
				ProbeEndpoints: []string{probe},
			}},
		}},
	})

	localNode := testkit.StartNode(t, runtimeinfra.Config{
		Name:        "network-surface-local",
		NodeProfile: transport.NodeProfileServiceNode,
		Boot:        runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Trust:       runtimeinfra.TrustConfig{Registry: testkit.DiscoveryTrustRegistry(t, remote.Snapshot().Ident.PublicKey)},
		Transport:   runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: transport.ReachabilityPrivateLAN},
		Data:        runtimeinfra.DataConfig{Dir: t.TempDir()}, Privacy: privacy.Receiver,
	})
	testkit.WaitForServiceMatchCount(t, 10*time.Second, remote, "echo", 1)
	imported := testkit.ImportRecordsFromNode(t, localNode, remote, "bootstrap", nil)
	require.NotEmpty(t, imported)

	client := testkit.NewArdentsClient(t, localNode)

	localNetwork := localNode.GetNetworkStatus()
	localDiscovery := localNode.GetDiscoveryStatus()
	localPresence := localNode.GetLocalPresence()
	localPeers := localNode.ListPeers()
	localRouteCandidates, localRoute, err := localNode.ListRouteCandidates(discoveryapi.ListRouteCandidatesQuery{Service: "echo"})
	require.NoError(t, err)

	rpcNetwork, err := client.GetNetworkStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetNetworkStatusRequest{}))
	require.NoError(t, err)
	rpcDiscovery, err := client.GetDiscoveryStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetDiscoveryStatusRequest{}))
	require.NoError(t, err)
	rpcPresence, err := client.GetLocalPresence(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetLocalPresenceRequest{}))
	require.NoError(t, err)
	rpcPeers, err := client.ListPeers(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ListPeersRequest{}))
	require.NoError(t, err)
	rpcRoutes, err := client.ListRouteCandidates(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ListRouteCandidatesRequest{Service: "echo"}))
	require.NoError(t, err)

	require.Equal(t, localNetwork.State, rpcNetwork.Msg.GetNetwork().GetState())
	require.Equal(t, localNetwork.Reason, rpcNetwork.Msg.GetNetwork().GetReason())
	require.Equal(t, localNetwork.Joined, rpcNetwork.Msg.GetNetwork().GetJoined())
	require.Equal(t, localNetwork.Reachable, rpcNetwork.Msg.GetNetwork().GetReachable())
	require.Equal(t, localNetwork.ActiveProfile, rpcNetwork.Msg.GetNetwork().GetActiveProfile())
	require.Equal(t, localNetwork.ActiveMode, rpcNetwork.Msg.GetNetwork().GetActiveMode())
	require.Equal(t, "service_node", localNetwork.NodeProfile)
	require.Equal(t, localNetwork.NodeProfile, rpcNetwork.Msg.GetNetwork().GetNodeProfile())
	require.Equal(t, "ardents-private/1", localNetwork.PrivacyProfile)
	require.Equal(t, "active", localNetwork.PrivacyState)
	require.Equal(t, "channel_grant_ready", localNetwork.PrivacySwitchReason)
	require.Equal(t, "steady", localNetwork.PrivacyRecoveryState)
	require.Empty(t, localNetwork.PrivacyErrors)
	require.Equal(t, localNetwork.PrivacyProfile, rpcNetwork.Msg.GetNetwork().GetPrivacyProfile())
	require.Equal(t, localNetwork.PrivacyState, rpcNetwork.Msg.GetNetwork().GetPrivacyState())
	require.Equal(t, localNetwork.PrivacySwitchReason, rpcNetwork.Msg.GetNetwork().GetPrivacySwitchReason())
	require.Equal(t, localNetwork.PrivacyRecoveryState, rpcNetwork.Msg.GetNetwork().GetPrivacyRecoveryState())
	require.Equal(t, localNetwork.PrivacyErrors, rpcNetwork.Msg.GetNetwork().GetPrivacyErrorCategories())

	require.Equal(t, localDiscovery.State, rpcDiscovery.Msg.GetDiscovery().GetState())
	require.Equal(t, localDiscovery.Reason, rpcDiscovery.Msg.GetDiscovery().GetReason())
	require.Equal(t, localDiscovery.LocalRecords, int(rpcDiscovery.Msg.GetDiscovery().GetLocalRecords()))
	require.Equal(t, localDiscovery.RemoteRecords, int(rpcDiscovery.Msg.GetDiscovery().GetRemoteRecords()))
	require.Equal(t, localDiscovery.TrustedRecords, int(rpcDiscovery.Msg.GetDiscovery().GetTrustedRecords()))
	require.Equal(t, localDiscovery.RejectedRecords, int(rpcDiscovery.Msg.GetDiscovery().GetRejectedRecords()))

	require.Equal(t, localPresence.Published, rpcPresence.Msg.GetPresence().GetPublished())
	require.Equal(t, localPresence.State, rpcPresence.Msg.GetPresence().GetState())
	require.Equal(t, localPresence.RecordID, rpcPresence.Msg.GetPresence().GetRecordId())
	require.Equal(t, localPresence.OperatorActionRequired, rpcPresence.Msg.GetPresence().GetOperatorActionRequired())

	require.Len(t, localPeers, 1)
	require.Len(t, rpcPeers.Msg.GetPeers(), 1)
	require.Equal(t, localPeers[0].NodeID, rpcPeers.Msg.GetPeers()[0].GetNodeId())
	require.Equal(t, localPeers[0].State, rpcPeers.Msg.GetPeers()[0].GetState())
	require.Equal(t, localPeers[0].Reachability, rpcPeers.Msg.GetPeers()[0].GetReachability())
	require.Equal(t, localPeers[0].Trust.Outcome, rpcPeers.Msg.GetPeers()[0].GetTrust().GetOutcome())

	require.NotEmpty(t, localRouteCandidates)
	require.NotEmpty(t, rpcRoutes.Msg.GetCandidates())
	require.Equal(t, localRoute.Outcome, rpcRoutes.Msg.GetRoute().GetOutcome())
	require.Equal(t, localRouteCandidates[0].Endpoint, rpcRoutes.Msg.GetCandidates()[0].GetEndpoint())
	require.Equal(t, localRouteCandidates[0].Service, rpcRoutes.Msg.GetCandidates()[0].GetService())
	require.Equal(t, localRouteCandidates[0].Usable, rpcRoutes.Msg.GetCandidates()[0].GetUsable())
}

func TestConnectRPCHostedServiceSurfaceMatchesLocalTruth(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "local-control-surface",
		ScenarioID:  "INT-SVC-SURFACE-001",
		Suite:       "integration",
		Tags:        []string{"integration", "local-control-surface", "workload", "hosted-service"},
		Speed:       "default",
		Environment: "local",
	})

	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	config, endpoint, probe := localControlReadyFixture(t)
	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "service-surface", NodeProfile: transport.NodeProfileServiceNode,
		Boot:      runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Transport: runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: transport.ReachabilityPrivateLAN},
		Data:      runtimeinfra.DataConfig{Dir: t.TempDir()}, Privacy: privacy.Sender,
		Workload: []runtimeinfra.WorkloadConfig{{
			ID:      "work.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  config,
			Desired: "running",
			Services: []runtimeinfra.ServiceConfig{{
				ID:             "svc.work.echo",
				Type:           "echo",
				Mode:           "NetworkPublished",
				Endpoints:      []string{endpoint},
				ProbeEndpoints: []string{probe},
			}},
		}},
	})
	testkit.WaitForServiceMatchCount(t, 10*time.Second, rt.Runtime, "echo", 1)

	client := testkit.NewArdentsClient(t, rt.Runtime)
	assertServiceSurface := func(wantPublished bool) {
		localHosted, err := rt.Hosting.GetHostedService("svc.work.echo")
		require.NoError(t, err)
		localList, err := rt.Hosting.ListHostedServices()
		require.NoError(t, err)
		localPublication, err := rt.Hosting.GetServicePublicationStatus("svc.work.echo")
		require.NoError(t, err)

		rpcHosted, err := client.GetHostedService(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetHostedServiceRequest{Id: "svc.work.echo"}))
		require.NoError(t, err)
		rpcList, err := client.ListHostedServices(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ListHostedServicesRequest{}))
		require.NoError(t, err)
		rpcPublication, err := client.GetServicePublicationStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetServicePublicationStatusRequest{Id: "svc.work.echo"}))
		require.NoError(t, err)

		require.Equal(t, localHosted.State, rpcHosted.Msg.GetService().GetState())
		require.Equal(t, localHosted.Published, rpcHosted.Msg.GetService().GetPublished())
		require.Equal(t, localHosted.RuntimeBacking, rpcHosted.Msg.GetService().GetRuntimeBacking())
		require.Equal(t, localHosted.Ready, rpcHosted.Msg.GetService().GetReady())
		require.Equal(t, localHosted.ExposureEligible, rpcHosted.Msg.GetService().GetExposureEligible())
		require.Equal(t, localHosted.Generation, rpcHosted.Msg.GetService().GetGeneration())
		require.Equal(t, localHosted.Publication.State, rpcHosted.Msg.GetService().GetPublication().GetState())
		require.Equal(t, localHosted.Publication.Reason, rpcHosted.Msg.GetService().GetPublication().GetReason())

		localListed := findHostedServiceSnapshot(t, localList, "svc.work.echo")
		rpcListed := findProtoHostedServiceSnapshot(t, rpcList.Msg.GetServices(), "svc.work.echo")
		require.Equal(t, localListed.RuntimeBacking, rpcListed.GetRuntimeBacking())
		require.Equal(t, localListed.DesiredPublication, rpcListed.GetDesiredPublication())
		require.Equal(t, localListed.Visibility, rpcListed.GetVisibility())
		require.Equal(t, localListed.Readiness, rpcListed.GetReadiness())
		require.Equal(t, localListed.Ready, rpcListed.GetReady())
		require.Equal(t, localListed.ExposureEligible, rpcListed.GetExposureEligible())
		require.Equal(t, localListed.Generation, rpcListed.GetGeneration())

		require.Equal(t, localPublication.State, rpcPublication.Msg.GetPublication().GetState())
		require.Equal(t, localPublication.Published, rpcPublication.Msg.GetPublication().GetPublished())
		require.Equal(t, localPublication.OperatorActionRequired, rpcPublication.Msg.GetPublication().GetOperatorActionRequired())

		require.Equal(t, wantPublished, localHosted.Published)
		require.Equal(t, wantPublished, rpcHosted.Msg.GetService().GetPublished())
	}

	assertServiceSurface(true)

	require.NoError(t, rt.Workload.Stop(context.Background(), "work.echo"))
	assertServiceSurface(false)
}

func TestConnectRPCDataTransferSurfaceMatchesLocalTruth(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "local-control-surface",
		ScenarioID:  "INT-DATA-SURFACE-001",
		Suite:       "integration",
		Tags:        []string{"integration", "local-control-surface", "data", "transfer"},
		Speed:       "default",
		Environment: "local",
	})

	sourceDir := t.TempDir()
	sourceStore := appdata.NewInDir(sourceDir)
	require.NoError(t, sourceStore.Load())
	key := []byte("0123456789abcdef0123456789abcdef")
	stored, err := sourceStore.StoreEncryptedBlob(appdata.Blob{MediaType: "application/octet-stream"}, []byte("network payload"), key, "")
	require.NoError(t, err)

	source := testkit.StartNode(t, runtimeinfra.Config{
		Name: "data-surface-source",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: sourceDir},
	})
	records, err := source.ListRecords()
	require.NoError(t, err)
	require.NotEmpty(t, records)

	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name:  "data-surface-requester",
		Boot:  runtimeinfra.BootConfig{Sources: append([]string(nil), records[0].EndpointList()...)},
		Trust: runtimeinfra.TrustConfig{Registry: testkit.DiscoveryTrustRegistry(t, source.Snapshot().Ident.PublicKey)},
		Data:  runtimeinfra.DataConfig{Dir: t.TempDir()},
	})
	client := testkit.NewArdentsClient(t, rt.Runtime)
	fetched, err := client.FetchBlob(context.Background(), testkit.AuthorizedRequest(&ardentsv1.FetchBlobRequest{Id: stored.Reference.String()}))
	require.NoError(t, err)
	require.Equal(t, stored.Reference.String(), fetched.Msg.GetReference())

	var localTransfers []transfer.Record
	var rpcTransfers *ardentsv1.ListTransfersResponse
	testkit.WaitForCondition(t, 5*time.Second, "transfer snapshots are visible through local and connectrpc surfaces", func() (bool, string) {
		localTransfers = rt.Transfers.List()
		resp, err := client.ListTransfers(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ListTransfersRequest{}))
		if err != nil {
			return false, err.Error()
		}
		rpcTransfers = resp.Msg
		localTransfer, ok := findLocalTransferByResource(localTransfers, stored.Reference.String())
		if !ok {
			return false, "local transfer not visible yet"
		}
		rpcTransfer, ok := findProtoTransferByResource(rpcTransfers.GetTransfers(), stored.Reference.String())
		if !ok {
			return false, "rpc transfer not visible yet"
		}
		if localTransfer.State != "completed" || rpcTransfer.GetState() != "completed" {
			return false, localTransfer.State + "/" + rpcTransfer.GetState()
		}
		return true, ""
	})

	localSources := rt.Data.ListBlobSources(stored.Reference.String())
	localTransfer, ok := findLocalTransferByResource(localTransfers, stored.Reference.String())
	require.True(t, ok)
	localGetTransfer, ok := rt.Transfers.Get(localTransfer.ID)
	require.True(t, ok)

	rpcSources, err := client.ListBlobSources(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ListBlobSourcesRequest{Id: stored.Reference.String()}))
	require.NoError(t, err)
	rpcGetTransfer, err := client.GetTransfer(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetTransferRequest{Id: localTransfer.ID}))
	require.NoError(t, err)

	require.Len(t, localSources, len(rpcSources.Msg.GetSources()))
	require.GreaterOrEqual(t, len(localSources), 2)
	localLocalSource, ok := findLocalBlobSourceByTransport(localSources, "local")
	require.True(t, ok)
	rpcLocalSource, ok := findProtoBlobSourceByTransport(rpcSources.Msg.GetSources(), "local")
	require.True(t, ok)
	localRemoteSource, ok := findLocalBlobSourceByTransport(localSources, "remote")
	require.True(t, ok)
	rpcRemoteSource, ok := findProtoBlobSourceByTransport(rpcSources.Msg.GetSources(), "remote")
	require.True(t, ok)
	require.Equal(t, localLocalSource.NodeID, rpcLocalSource.GetNodeId())
	require.Equal(t, localLocalSource.ContentReference.String(), rpcLocalSource.GetContentReference())
	require.Equal(t, localRemoteSource.NodeID, rpcRemoteSource.GetNodeId())
	require.Equal(t, localRemoteSource.Usable, rpcRemoteSource.GetUsable())

	require.Equal(t, localTransfer.ID, rpcGetTransfer.Msg.GetTransfer().GetId())
	require.Equal(t, localGetTransfer.State, rpcGetTransfer.Msg.GetTransfer().GetState())
	require.Equal(t, localGetTransfer.ResourceID, rpcGetTransfer.Msg.GetTransfer().GetResourceId())
	require.Equal(t, localGetTransfer.Direction, rpcGetTransfer.Msg.GetTransfer().GetDirection())
	require.Equal(t, localGetTransfer.Peer, rpcGetTransfer.Msg.GetTransfer().GetPeer())
}

func TestConnectRPCDataSurfaceMarksStaleRemoteSourceUnusable(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "local-control-surface",
		ScenarioID:  "INT-DATA-SURFACE-001",
		Suite:       "integration",
		Tags:        []string{"integration", "local-control-surface", "data", "transfer"},
		Speed:       "default",
		Environment: "local",
	})

	dataDir := t.TempDir()
	store := appdata.NewInDir(dataDir)
	require.NoError(t, store.Load())
	blob, err := store.AnnounceRemoteBlob(appdata.Blob{
		ID:        "blob-stale-remote",
		CID:       "blob-stale-remote",
		MediaType: "application/octet-stream",
		State:     "available-remote",
	})
	require.NoError(t, err)
	_, err = store.ObserveBlobSource(blob.Reference.String(), appdata.BlobSourceRecord{
		NodeID:     "p_remote_stale",
		Trust:      appdata.SourceTrust{State: "ready", Outcome: "usable", Valid: true, Trusted: true, Usable: true},
		Usable:     true,
		Transport:  "remote",
		LastSeenAt: time.Now().UTC().Add(-24 * time.Hour),
		Reason:     "trusted remote source answered blob fetch",
	})
	require.NoError(t, err)

	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "data-surface-stale-source",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dataDir},
	})
	client := testkit.NewArdentsClient(t, rt.Runtime)
	localSources := rt.Data.ListBlobSources(blob.Reference.String())
	require.Len(t, localSources, 1)
	require.Equal(t, "p_remote_stale", localSources[0].NodeID)
	require.False(t, localSources[0].Usable)
	require.False(t, localSources[0].Trust.Usable)
	require.Contains(t, localSources[0].Reason, "stale")

	rpcSources, err := client.ListBlobSources(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ListBlobSourcesRequest{Id: blob.Reference.String()}))
	require.NoError(t, err)
	require.Len(t, rpcSources.Msg.GetSources(), 1)
	require.Equal(t, localSources[0].NodeID, rpcSources.Msg.GetSources()[0].GetNodeId())
	require.Equal(t, localSources[0].Usable, rpcSources.Msg.GetSources()[0].GetUsable())
	require.Contains(t, rpcSources.Msg.GetSources()[0].GetReason(), "stale")
}

func TestConnectRPCDiagnosticsSurfaceMatchesLocalTruth(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "local-control-surface",
		ScenarioID:  "INT-DIAG-SURFACE-001",
		Suite:       "integration",
		Tags:        []string{"integration", "local-control-surface", "diagnostics"},
		Speed:       "default",
		Environment: "local",
	})

	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "diagnostics-surface",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
		Workload: []runtimeinfra.WorkloadConfig{{
			ID:      "work.invalid",
			Kind:    "unsupported",
			Owner:   "node",
			Desired: "running",
			Services: []runtimeinfra.ServiceConfig{{
				ID:        "svc.work.invalid",
				Type:      "echo",
				Mode:      "NetworkPublished",
				Endpoints: []string{"tcp://127.0.0.1:9000"},
			}},
		}},
	})

	client := testkit.NewArdentsClient(t, rt.Runtime)
	localHealth := rt.Diagnostics.GetHealthSummary()
	localExplanation := rt.Diagnostics.ExplainFailure("service", "svc.work.invalid")
	rpcHealth, err := client.GetHealthSummary(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetHealthSummaryRequest{}))
	require.NoError(t, err)
	rpcExplanation, err := client.ExplainFailure(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ExplainFailureRequest{
		Scope:      "service",
		ResourceId: "svc.work.invalid",
	}))
	require.NoError(t, err)
	rpcEvents, err := client.ListRecentEvents(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ListRecentEventsRequest{Limit: 2}))
	require.NoError(t, err)
	localEvents, _ := rt.Diagnostics.ListRecentEvents(2, "")

	require.Equal(t, localHealth.State, rpcHealth.Msg.GetHealth().GetState())
	require.NotNil(t, localHealth.PrimaryReason)
	require.Equal(t, localHealth.PrimaryReason.Code, rpcHealth.Msg.GetHealth().GetPrimaryReason().GetCode())
	require.Equal(t, localHealth.PrimaryReason.Domain, rpcHealth.Msg.GetHealth().GetPrimaryReason().GetDomain())
	require.Equal(t, localHealth.OperatorActionRequired, rpcHealth.Msg.GetHealth().GetOperatorActionRequired())

	require.Equal(t, localExplanation.Scope, rpcExplanation.Msg.GetExplanation().GetScope())
	require.Equal(t, localExplanation.ResourceID, rpcExplanation.Msg.GetExplanation().GetResourceId())
	require.Equal(t, localExplanation.State, rpcExplanation.Msg.GetExplanation().GetState())
	require.NotNil(t, localExplanation.Reason)
	require.Equal(t, localExplanation.Reason.Code, rpcExplanation.Msg.GetExplanation().GetReason().GetCode())
	require.Equal(t, localExplanation.Reason.Recovery, rpcExplanation.Msg.GetExplanation().GetReason().GetRecovery())

	require.NotEmpty(t, localEvents)
	require.NotEmpty(t, rpcEvents.Msg.GetEvents())
	require.Equal(t, len(localEvents), len(rpcEvents.Msg.GetEvents()))
	require.Equal(t, localEvents[len(localEvents)-1].Seq, rpcEvents.Msg.GetEvents()[len(rpcEvents.Msg.GetEvents())-1].GetSeq())
	require.Equal(t, localEvents[len(localEvents)-1].Type, rpcEvents.Msg.GetEvents()[len(rpcEvents.Msg.GetEvents())-1].GetType())
}

func findHostedServiceSnapshot(t *testing.T, services []hosting.ServiceSnapshot, id string) hosting.ServiceSnapshot {
	t.Helper()
	for _, item := range services {
		if item.ID == id {
			return item
		}
	}
	require.FailNow(t, "expected hosted service in local result", "service id=%s", id)
	return hosting.ServiceSnapshot{}
}

func findProtoHostedServiceSnapshot(t *testing.T, services []*ardentsv1.HostedServiceSnapshot, id string) *ardentsv1.HostedServiceSnapshot {
	t.Helper()
	for _, item := range services {
		if item.GetId() == id {
			return item
		}
	}
	require.FailNow(t, "expected hosted service in rpc result", "service id=%s", id)
	return nil
}

func findLocalTransferByResource(transfers []transfer.Record, resourceID string) (transfer.Record, bool) {
	for _, item := range transfers {
		if item.ResourceID == resourceID {
			return item, true
		}
	}
	return transfer.Record{}, false
}

func findProtoTransferByResource(transfers []*ardentsv1.TransferSnapshot, resourceID string) (*ardentsv1.TransferSnapshot, bool) {
	for _, item := range transfers {
		if item.GetResourceId() == resourceID {
			return item, true
		}
	}
	return nil, false
}

func findLocalBlobSourceByTransport(sources []appdata.BlobSourceRecord, transport string) (appdata.BlobSourceRecord, bool) {
	for _, item := range sources {
		if item.Transport == transport {
			return item, true
		}
	}
	return appdata.BlobSourceRecord{}, false
}

func findProtoBlobSourceByTransport(sources []*ardentsv1.BlobSourceSnapshot, transport string) (*ardentsv1.BlobSourceSnapshot, bool) {
	for _, item := range sources {
		if item.GetTransport() == transport {
			return item, true
		}
	}
	return nil, false
}
