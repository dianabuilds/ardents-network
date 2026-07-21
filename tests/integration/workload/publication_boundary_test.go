//go:build integration

package workload_test

import (
	"context"
	"testing"
	"time"

	runtimeinfra "ardents/internal/daemon"
	transport "ardents/internal/network"
	"ardents/internal/workload/execution"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestPublicationBoundaryWithdrawsServiceWhenRuntimeStops(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	config, endpoint, probe := workloadReadyFixture(t)
	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "integration-runtime-stop", NodeProfile: transport.NodeProfileServiceNode,
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
	item, err := rt.Workload.Get("work.echo")
	require.NoError(t, err)
	require.Equal(t, execution.ObservedRunning, item.Observed)
	require.Len(t, item.PublishedServices, 1)
	require.True(t, item.PublishedServices[0].Published)

	res, err := rt.Discovery.ResolveService("echo")
	require.NoError(t, err)
	require.NotEmpty(t, res.Matches)

	require.NoError(t, rt.Workload.Stop(context.Background(), "work.echo"))

	item, err = rt.Workload.Get("work.echo")
	require.NoError(t, err)
	require.Equal(t, execution.ObservedStopped, item.Observed)
	require.Len(t, item.PublishedServices, 1)
	require.False(t, item.PublishedServices[0].Published)
	require.NotEmpty(t, item.PublishedServices[0].Reason)

	res, err = rt.Discovery.ResolveService("echo")
	require.NoError(t, err)
	require.Equal(t, "not_found", res.Outcome)
	require.Len(t, res.Matches, 0)

	hosted, err := rt.Hosting.GetHostedService("svc.work.echo")
	require.NoError(t, err)
	require.False(t, hosted.Published)
	require.NotEmpty(t, hosted.Reason)
}

func TestPublicationBoundaryPolicyDeniedStatus(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "integration-policy-publish",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
		Policy: runtimeinfra.PolicyConfig{
			DisableNetworkPublishedServices: true,
		},
		Workload: []runtimeinfra.WorkloadConfig{{
			ID:      "work.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  testkit.HelperProcessConfig(t, "sleep"),
			Desired: "running",
			Services: []runtimeinfra.ServiceConfig{{
				ID:        "svc.work.echo",
				Type:      "echo",
				Mode:      "NetworkPublished",
				Endpoints: []string{"quic://echo:9000"},
			}},
		}},
	})
	item, err := rt.Workload.Get("work.echo")
	require.NoError(t, err)
	require.Len(t, item.PublishedServices, 1)
	require.False(t, item.PublishedServices[0].Published)
	require.NotEmpty(t, item.PublishedServices[0].Reason)

	hosted, err := rt.Hosting.GetHostedService("svc.work.echo")
	require.NoError(t, err)
	require.False(t, hosted.Published)
	require.NotEmpty(t, hosted.Reason)

	res, err := rt.Discovery.ResolveService("echo")
	require.NoError(t, err)
	require.Equal(t, "not_found", res.Outcome)
	require.Len(t, res.Matches, 0)
}
