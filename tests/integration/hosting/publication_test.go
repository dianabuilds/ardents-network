//go:build integration

package hosting_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	runtimeprocess "ardents/internal/daemon"
	discoveryapi "ardents/internal/discovery"
	networkapi "ardents/internal/network"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

//goland:noinspection ALL
func TestPublishedServiceResolvesAndConnectsAcrossRealWakuNodes(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{Layer: testkit.LayerIntegration, Domain: "hosted-services", ScenarioID: "HSI-002",
		Suite: "integration", Tags: []string{"integration", "hosted-services", "publication", "waku"},
		Speed: "slow", Environment: "linux-container"})
	now := time.Now().UTC().Truncate(time.Second)
	privacy := testkit.NewDiscoveryPrivacyFixture(t, now)
	host := privateContainerIPv4(t)
	port := reservePort(t)
	advertised := "http://" + net.JoinHostPort(host, fmt.Sprint(port)) + "/ready"
	probe := fmt.Sprintf("http://127.0.0.1:%d/ready", port)

	remote := testkit.StartNode(t, runtimeprocess.Config{
		Name: "published-service-remote", NodeProfile: networkapi.NodeProfileServiceNode,
		Boot:      runtimeprocess.BootConfig{Sources: []string{"local://bootstrap"}},
		Transport: runtimeprocess.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: networkapi.ReachabilityPrivateLAN},
		Data:      runtimeprocess.DataConfig{Dir: t.TempDir()}, Privacy: privacy.Sender, DiscoveryRefreshInterval: 50 * time.Millisecond,
		Workload: []runtimeprocess.WorkloadConfig{{ID: "work.published", Kind: "service", Owner: "node", Desired: "running",
			Config: readinessHelperConfig(t, fmt.Sprintf("0.0.0.0:%d", port)), Services: []runtimeprocess.ServiceConfig{{
				ID: "svc.published", Type: "http", Mode: "NetworkPublished",
				Endpoints: []string{advertised}, ProbeEndpoints: []string{probe},
			}}}},
	})

	testkit.WaitForCondition(t, 5*time.Second, "current ready service is locally published", func() (bool, string) {
		result, err := remote.ResolveRecord("svc.published", "service")
		if err != nil {
			return false, err.Error()
		}
		if result.Outcome == "found" && len(result.Record.Endpoints) == 1 {
			return true, ""
		}
		hosted, hostedErr := testkit.Hosting(remote).GetHostedService("svc.published")
		publication, publicationErr := testkit.Hosting(remote).GetServicePublicationStatus("svc.published")
		network := remote.GetNetworkStatus()
		return false, fmt.Sprintf("resolve=%s hosted=%+v hosted_err=%v publication=%+v publication_err=%v network=%+v",
			result.Outcome, hosted, hostedErr, publication, publicationErr, network)
	})
	remoteRecords, err := remote.ListRecords()
	require.NoError(t, err)
	bootstrap := nodeRecordEndpoints(remoteRecords)
	require.NotEmpty(t, bootstrap)

	local := testkit.StartNode(t, runtimeprocess.Config{
		Name: "published-service-local", NodeProfile: networkapi.NodeProfileServiceNode,
		Boot:      runtimeprocess.BootConfig{Sources: bootstrap},
		Transport: runtimeprocess.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: networkapi.ReachabilityPrivateLAN},
		Data:      runtimeprocess.DataConfig{Dir: t.TempDir()}, Privacy: privacy.Receiver,
		DiscoveryRefreshInterval: 50 * time.Millisecond,
	})
	testkit.WaitForCondition(t, 5*time.Second, "second node resolves signed service record", func() (bool, string) {
		result, resolveErr := local.ResolveRecord("svc.published", "service")
		if resolveErr != nil {
			return false, resolveErr.Error()
		}
		return result.Outcome == "found" && len(result.Record.Endpoints) == 1 && result.Record.Endpoints[0] == advertised, result.Outcome
	})
	requireServiceRequest(t, advertised)

	require.NoError(t, testkit.Workloads(remote).Stop(context.Background(), "work.published"))
	testkit.WaitForCondition(t, 5*time.Second, "backing exit withdraws remote service record", func() (bool, string) {
		result, resolveErr := local.ResolveRecord("svc.published", "service")
		if resolveErr != nil {
			return false, resolveErr.Error()
		}
		return result.Outcome == "withdrawn" && len(result.Record.Endpoints) == 0, result.Outcome
	})
	require.Eventually(t, func() bool {
		client := &http.Client{Timeout: 150 * time.Millisecond}
		response, requestErr := client.Get(advertised)
		if response != nil {
			requestErr = errors.Join(requestErr, response.Body.Close())
		}
		return requestErr != nil
	}, 3*time.Second, 50*time.Millisecond)
}

func privateContainerIPv4(t *testing.T) string {
	t.Helper()
	addresses, err := net.InterfaceAddrs()
	require.NoError(t, err)
	for _, address := range addresses {
		ip, _, parseErr := net.ParseCIDR(address.String())
		if parseErr == nil && ip.To4() != nil && ip.IsPrivate() && !ip.IsLoopback() {
			return ip.String()
		}
	}
	t.Fatal("Linux test container has no private IPv4 address")
	return ""
}

func reservePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	require.NoError(t, err)
	defer func() { require.NoError(t, listener.Close()) }()
	return listener.Addr().(*net.TCPAddr).Port
}

func nodeRecordEndpoints(records []discoveryapi.CatalogRecordSnapshot) []string {
	for _, record := range records {
		if record.Kind == "node" && len(record.Endpoints) > 0 {
			return append([]string(nil), record.Endpoints...)
		}
	}
	return nil
}

func requireServiceRequest(t *testing.T, endpoint string) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get(endpoint)
	require.NoError(t, err)
	defer func() { require.NoError(t, response.Body.Close()) }()
	require.Equal(t, http.StatusNoContent, response.StatusCode)
}
