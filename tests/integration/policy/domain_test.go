//go:build integration

package policy_test

import (
	"context"
	"strings"
	"testing"
	"time"

	appdata "ardents/internal/content"
	runtimeinfra "ardents/internal/daemon"
	diagapi "ardents/internal/diagnostics"
	transport "ardents/internal/network"
	workloadapi "ardents/internal/workload"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestPolicyRejectsWorkloadByCapability(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "policy",
		ScenarioID:  "POI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "policy"},
		Speed:       "default",
		Environment: "local",
	})
	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "policy-admission",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
		Policy: runtimeinfra.PolicyConfig{
			DeniedCapabilities: []string{"net-bind"},
		},
	})

	err := testkit.Workloads(n).Register(context.Background(), workloadapi.SpecSnapshot{
		ID:           "work.blocked",
		Kind:         "service",
		Owner:        "node",
		Config:       testkit.HelperProcessConfig(t, "sleep"),
		Desired:      "present",
		Capabilities: []string{"net-bind"},
	})
	require.Falsef(t, err == nil || !strings.
		Contains(err.Error(),

			"policy_admission_denied",
		), "error = %v, want policy admission denial", err)
	{

		snapshot := n.Snapshot()
		require.Falsef(t, snapshot.Policy.State !=
			"enforced", "policy state = %q, want enforced", snapshot.Policy.State)
	}
	{

		_, err := n.PublishBlob(appdata.PublishBlobCommand{
			Blob:    appdata.Blob{MediaType: "text/plain"},
			Payload: []byte("allowed after deny"),
		})
		require.NoErrorf(t, err, "publish allowed blob after deny: %v", err)
	}
	{

		snapshot := n.Snapshot()
		require.Falsef(t, snapshot.Policy.State !=
			"enforced", "policy state after allowed operation = %q, want enforced", snapshot.Policy.State)
	}
	require.True(t, hasPolicyDeniedEvent(testkit.Diagnostics(n).DiagnosticsSnapshot()), "expected policy.denied diagnostics event")

}

func TestPolicyBlocksHostedServicePublicationAndSurfaceProjection(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "policy",
		ScenarioID:  "POI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "policy"},
		Speed:       "default",
		Environment: "local",
	})
	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "policy-publish",
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

	res, err := n.ResolveService("echo")
	require.NoErrorf(t, err, "resolve service: %v", err)
	require.Falsef(t, len(res.Matches) != 0, "matches = %d, want 0 when publication denied", len(res.Matches))

	item, err := testkit.Workloads(n).Get("work.echo")
	require.NoErrorf(t, err, "get workload: %v", err)
	require.Falsef(t, len(item.PublishedServices) != 1, "published services = %d, want 1", len(item.PublishedServices))
	require.False(t, item.PublishedServices[0].Published, "expected workload published service to be false when policy denies publication")
	require.False(t, item.PublishedServices[0].Reason == "", "expected workload publication denial reason")

	hosted, err := testkit.Hosting(n).GetHostedService("svc.work.echo")
	require.NoErrorf(t, err, "get hosted service: %v", err)
	require.False(t, hosted.Published, "expected hosted service published to be false when policy denies publication")
	require.False(t, hosted.Reason == "", "expected hosted service denial reason")
	require.True(t, hasPolicyDeniedEvent(testkit.Diagnostics(n).DiagnosticsSnapshot()), "expected policy.denied diagnostics event")

}

func TestPolicyRejectsBlobRetentionAndPinning(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "policy",
		ScenarioID:  "POI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "policy"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()
	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "policy-retention",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
		Policy: runtimeinfra.PolicyConfig{
			DisableLocalBlobRetention: true,
			DisableBlobPinning:        true,
		},
	})

	blob, err := n.PublishBlob(appdata.PublishBlobCommand{
		Blob:    appdata.Blob{MediaType: "text/plain"},
		Payload: []byte("policy payload"),
	})
	require.NoErrorf(t, err, "publish blob: %v", err)
	{

		_, err := n.RetainBlob(blob.ID, time.Now().UTC().Add(time.Hour))
		require.Falsef(t, err == nil || !strings.
			Contains(err.Error(),

				"policy_retention_denied",
			), "retain error = %v, want policy retention denial", err)
	}
	{

		_, err := n.PinBlob(blob.ID)
		require.Falsef(t, err == nil || !strings.
			Contains(err.Error(),

				"policy_pin_denied",
			), "pin error = %v, want policy pin denial", err)
	}
	require.True(t, hasPolicyDeniedEvent(testkit.Diagnostics(n).DiagnosticsSnapshot()), "expected policy.denied diagnostics event")

}

func TestPolicyRejectsPeerBlobReserving(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "policy",
		ScenarioID:  "POI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "policy"},
		Speed:       "default",
		Environment: "local",
	})
	sourceDir := t.TempDir()
	sourceStore := appdata.NewInDir(sourceDir)
	{
		err := sourceStore.Load()
		require.NoErrorf(t, err, "load source data store: %v", err)
	}

	stored, err := sourceStore.StoreEncryptedBlob(appdata.Blob{MediaType: "application/octet-stream"}, []byte("network payload"), []byte("0123456789abcdef0123456789abcdef"), "")
	require.NoErrorf(t, err, "store encrypted source blob: %v", err)

	source := testkit.StartNode(t, runtimeinfra.Config{
		Name: "source-no-reserve",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: sourceDir},
		Policy: runtimeinfra.PolicyConfig{
			DisablePeerBlobReserving: true,
		},
	})

	records, err := source.ListRecords()
	require.NoErrorf(t, err, "list source records: %v", err)
	require.False(t, len(records) == 0, "expected source discovery records")

	requester := testkit.StartNode(t, runtimeinfra.Config{
		Name:  "requester-no-reserve",
		Boot:  runtimeinfra.BootConfig{Sources: append([]string(nil), records[0].EndpointList()...)},
		Trust: runtimeinfra.TrustConfig{Registry: testkit.DiscoveryTrustRegistry(t, source.Snapshot().Ident.PublicKey)},
		Data:  runtimeinfra.DataConfig{Dir: t.TempDir()},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	{
		_, err := requester.FetchBlob(ctx, stored.ID)
		require.Falsef(t, err == nil || !strings.
			Contains(err.Error(),

				"policy_reserving_denied",
			), "error = %v, want terminal policy peer reserve denial", err)
	}
	{

		_, ok := testkit.Content(requester).GetBlob(stored.ID)
		require.False(t, ok, "expected requester to keep blob unavailable locally")
	}
	require.True(t, hasPolicyDeniedEvent(testkit.Diagnostics(source).DiagnosticsSnapshot()), "expected source node to emit policy.denied diagnostics event")

}

func TestPolicyRejectsRouteUse(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "policy",
		ScenarioID:  "POI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "policy"},
		Speed:       "default",
		Environment: "local",
	})
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	config, endpoint, probe := policyReadyFixture(t)
	localNode := testkit.StartNode(t, runtimeinfra.Config{
		Name: "local-route-policy",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()}, Privacy: privacy.Receiver,
		Policy: runtimeinfra.PolicyConfig{
			DeniedRouteSchemes: []string{"http"},
		},
	})
	remoteNode := testkit.StartNode(t, runtimeinfra.Config{
		Name: "remote-route-policy", NodeProfile: transport.NodeProfileServiceNode,
		Boot:      runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Transport: runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: transport.ReachabilityPrivateLAN},
		Data:      runtimeinfra.DataConfig{Dir: t.TempDir()}, Privacy: privacy.Sender,
		Workload: []runtimeinfra.WorkloadConfig{{
			ID:      "work.echo.route",
			Kind:    "service",
			Owner:   "node",
			Config:  config,
			Desired: "running",
			Services: []runtimeinfra.ServiceConfig{{
				ID:             "svc.echo.policy",
				Type:           "echo",
				Mode:           "NetworkPublished",
				Endpoints:      []string{endpoint},
				ProbeEndpoints: []string{probe},
			}},
		}},
	})
	testkit.WaitForServiceMatchCount(t, 10*time.Second, remoteNode, "echo", 1)

	records, err := remoteNode.ListRecords()
	require.NoErrorf(t, err, "list remote records: %v", err)

	imported := false
	for _, record := range records {
		if record.Kind() != "service" || record.Service == nil || record.Service.Type != "echo" {
			continue
		}
		{
			_, err := localNode.ImportRecord(record)
			require.NoErrorf(t, err, "import record: %v", err)
		}

		imported = true
	}
	require.True(t, imported, "expected imported service record")

	res, err := localNode.ResolveService("echo")
	require.NoErrorf(t, err, "resolve service: %v", err)
	require.Falsef(t, res.Route.Outcome == "usable", "route outcome = %q, want denied by policy", res.Route.Outcome)
	require.True(t, hasPolicyDeniedEvent(testkit.Diagnostics(localNode).DiagnosticsSnapshot()), "expected policy.denied diagnostics event")

}

func hasPolicyDeniedEvent(diag diagapi.DiagSnapshot) bool {
	for _, evt := range diag.RecentEvents {
		if evt.Domain == "policy" && evt.Type == "denied" {
			return true
		}
	}
	return false
}
