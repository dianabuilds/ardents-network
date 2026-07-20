//go:build integration

package workload_test

import (
	"context"
	"testing"
	"time"

	transport "ardents/internal/network/api"
	runtimeinfra "ardents/internal/runtime/process"
	workloadapi "ardents/internal/workload/api"
	"ardents/tests/testkit"

	ardentsv1 "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestLocalWorkloadManagementFlow(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-003",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "local",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
	})
	err := rt.Workload.RegisterWorkloadContext(context.Background(), workloadapi.WorkloadSpecSnapshot{
		ID:      "work.echo",
		Kind:    "service",
		Owner:   "node",
		Config:  testkit.HelperProcessConfig(t, "sleep"),
		Desired: "present",
	})
	require.NoError(t, err)

	require.NoError(t, rt.Workload.StartWorkloadContext(context.Background(), "work.echo"))

	items, err := rt.Workload.ListWorkloads()
	require.NoError(t, err)
	require.NotEmpty(t, items)
	require.Equal(t, "running", items[0].Observed)
	require.Empty(t, items[0].PublishedServices)

	item, err := rt.Workload.GetWorkloadStatus("work.echo")
	require.NoError(t, err)
	require.Equal(t, "running", item.Observed)
	require.Equal(t, "running", item.Spec.Desired)

	require.NoError(t, rt.Workload.StopWorkloadContext(context.Background(), "work.echo"))
	item, err = rt.Workload.GetWorkloadStatus("work.echo")
	require.NoError(t, err)
	require.Equal(t, "stopped", item.Observed)
	require.Equal(t, "stopped", item.Spec.Desired)

	require.NoError(t, rt.Workload.RestartWorkloadContext(context.Background(), "work.echo"))
	item, err = rt.Workload.GetWorkloadStatus("work.echo")
	require.NoError(t, err)
	require.Equal(t, "running", item.Observed)
	require.Equal(t, "running", item.Spec.Desired)
}

func TestLocalWorkloadStartFailureIsObservable(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-003",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "local-workload-start-fail",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
	})
	err := rt.Workload.RegisterWorkloadContext(context.Background(), workloadapi.WorkloadSpecSnapshot{
		ID:      "work.invalid.start",
		Kind:    "unsupported",
		Owner:   "tenant",
		Desired: "present",
	})
	require.NoError(t, err)

	err = rt.Workload.StartWorkloadContext(context.Background(), "work.invalid.start")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported")

	item, err := rt.Workload.GetWorkloadStatus("work.invalid.start")
	require.NoError(t, err)
	require.Equal(t, "failed", item.Observed)
}

func TestLocalWorkloadRegisterFailsWhenNodeStopped(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-003",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "local-workload-stopped",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
	})
	require.NoError(t, n.Stop(context.Background()))

	err := n.RegisterWorkloadContext(context.Background(), workloadapi.WorkloadSpecSnapshot{
		ID:      "work.after.stop",
		Kind:    "service",
		Owner:   "node",
		Desired: "running",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "stopped")

	items, err := n.ListWorkloads()
	require.NoError(t, err)
	require.Len(t, items, 0)
}

func TestLocalResolveWorkloadServiceType(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-003",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	config, endpoint, probe := workloadReadyFixture(t)
	localNode := testkit.StartNode(t, runtimeinfra.Config{
		Name: "local",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()}, Privacy: privacy.Receiver,
	})
	remoteNode := testkit.StartNode(t, runtimeinfra.Config{
		Name: "remote", NodeProfile: transport.NodeProfileServiceNode,
		Boot:      runtimeinfra.BootConfig{Sources: []string{"remote://bootstrap"}},
		Transport: runtimeinfra.NodeTransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: transport.ReachabilityPrivateLAN},
		Data:      runtimeinfra.NodeDataConfig{Dir: t.TempDir()}, Privacy: privacy.Sender,
		Workload: []runtimeinfra.NodeWorkloadConfig{{
			ID:      "work.remote.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  config,
			Desired: "running",
			Services: []runtimeinfra.NodeServiceConfig{{
				ID:             "svc.remote.echo",
				Type:           "echo",
				Mode:           "NetworkPublished",
				Endpoints:      []string{endpoint},
				ProbeEndpoints: []string{probe},
			}},
		}},
	})
	testkit.WaitForServiceMatchCount(t, 10*time.Second, remoteNode, "echo", 1)

	records, err := remoteNode.ListRecords()
	require.NoError(t, err)
	for _, record := range records {
		if record.Kind != "service" {
			continue
		}
		record.Source = "bootstrap"
		_, err := localNode.ImportRecord(record)
		require.NoError(t, err)
	}

	res, err := localNode.ResolveService("echo")
	require.NoError(t, err)
	require.Equal(t, "not_usable", res.Outcome)
	require.NotEmpty(t, res.Matches)
	require.NotNil(t, res.Route.Selected)
}

func TestConnectAPIWorkloadRoundTrip(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-003",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "connect",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
	})
	client := testkit.NewArdentsClient(t, n)
	_, err := client.RegisterWorkload(context.Background(), testkit.AuthorizedRequest(&ardentsv1.RegisterWorkloadRequest{
		Spec: &ardentsv1.WorkloadSpecSnapshot{
			Id:      "work.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  testkit.HelperProcessConfig(t, "sleep"),
			Desired: "present",
		},
	}))
	require.NoError(t, err)

	started, err := client.StartWorkload(context.Background(), testkit.AuthorizedRequest(&ardentsv1.StartWorkloadRequest{Id: "work.echo"}))
	require.NoError(t, err)
	require.Equal(t, "running", started.Msg.GetWorkload().GetObserved())

	got, err := client.GetWorkloadStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetWorkloadStatusRequest{Id: "work.echo"}))
	require.NoError(t, err)
	require.Equal(t, "running", got.Msg.GetSpec().GetDesired())

	_, err = client.StopWorkload(context.Background(), testkit.AuthorizedRequest(&ardentsv1.StopWorkloadRequest{Id: "work.echo"}))
	require.NoError(t, err)
	got, err = client.GetWorkloadStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetWorkloadStatusRequest{Id: "work.echo"}))
	require.NoError(t, err)
	require.Equal(t, "stopped", got.Msg.GetObserved())
	require.Equal(t, "stopped", got.Msg.GetSpec().GetDesired())
}

func TestConnectAPIRejectsDuplicateWorkloadRegistrationWithoutStateDrift(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-003",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "connect-duplicate",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
	})
	client := testkit.NewArdentsClient(t, n)

	spec := &ardentsv1.WorkloadSpecSnapshot{
		Id:      "work.echo",
		Kind:    "service",
		Owner:   "node",
		Config:  testkit.HelperProcessConfig(t, "sleep"),
		Desired: "running",
		Services: []*ardentsv1.PublishedServiceSnapshot{{
			Id:    "svc.work.echo",
			Type:  "echo",
			Owner: "work.echo",
			Mode:  "NetworkPublished",
		}},
	}
	_, err := client.RegisterWorkload(context.Background(), testkit.AuthorizedRequest(&ardentsv1.RegisterWorkloadRequest{Spec: spec}))
	require.NoError(t, err)

	_, err = client.RegisterWorkload(context.Background(), testkit.AuthorizedRequest(&ardentsv1.RegisterWorkloadRequest{
		Spec: &ardentsv1.WorkloadSpecSnapshot{
			Id:      "work.echo",
			Kind:    "unsupported",
			Owner:   "node",
			Config:  testkit.HelperProcessConfig(t, "sleep"),
			Desired: "present",
		},
	}))
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeAlreadyExists, connectErr.Code())

	got, err := client.GetWorkloadStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetWorkloadStatusRequest{Id: "work.echo"}))
	require.NoError(t, err)
	require.Equal(t, "running", got.Msg.GetObserved())
	require.Equal(t, "running", got.Msg.GetSpec().GetDesired())
	require.Len(t, got.Msg.GetSpec().GetServices(), 1)
	require.Equal(t, "work.echo", got.Msg.GetSpec().GetServices()[0].GetOwner())
}
