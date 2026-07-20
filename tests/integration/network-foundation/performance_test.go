//go:build integration

package networkfoundation_test

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"testing"
	"time"

	transport "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

const (
	performanceBatchSize = 16
	startupLimit         = 15 * time.Second
	batchLimit           = 5 * time.Second
	deliveryP95Limit     = 2 * time.Second
	storeQueryLimit      = 5 * time.Second
	shutdownLimit        = 5 * time.Second
)

type performanceBaseline struct {
	ctx                           context.Context
	cancel                        context.CancelFunc
	sender                        *networkprivacy.Channel
	contentTopic                  string
	remote, local                 transport.Service
	remoteStopped, localStopped   bool
	events                        <-chan transport.Envelope
	sentinel                      networkprivacy.SealedEnvelope
	connections                   map[string]int
	remoteStartup, localReadiness time.Duration
	batchDuration, deliveryP95    time.Duration
	storeDuration                 time.Duration
	localShutdown, remoteShutdown time.Duration
	throughput                    float64
}

func TestTransportPerformanceSafetyBaseline(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "network-foundation", ScenarioID: "NFI-007",
		Suite: "integration", Tags: []string{"integration", "network", "performance"},
		Speed: "default", Environment: "local",
	})
	baseline := newPerformanceBaseline(t)
	defer baseline.cancel()
	baseline.startRemote(scenario)
	baseline.startLocal(scenario)
	baseline.checkConnections(scenario)
	baseline.deliverBatch(scenario)
	baseline.checkRelayMetrics(scenario)
	baseline.fetchStore(scenario)
	baseline.stopTransports(scenario)
	baseline.logMetrics(scenario)
}

func newPerformanceBaseline(t *testing.T) *performanceBaseline {
	t.Helper()
	testkit.ConfigureLoopbackTransport(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	now := time.Now().UTC().Truncate(time.Second)
	fixture := newRelayPrivacyFixture(t, now)
	sender, _ := fixture.channels(t, now)
	contentTopic, err := sender.ContentTopic()
	require.NoError(t, err)
	baseline := &performanceBaseline{
		ctx: ctx, cancel: cancel, sender: sender, contentTopic: contentTopic,
		remote: newPerformanceTransport(t, "remote"), local: newPerformanceTransport(t, "local"),
		connections: make(map[string]int, 4),
	}
	t.Cleanup(func() { baseline.cleanup() })
	return baseline
}

func (b *performanceBaseline) cleanup() {
	if !b.localStopped {
		_ = b.local.Stop(context.Background())
	}
	if !b.remoteStopped {
		_ = b.remote.Stop(context.Background())
	}
}

func (b *performanceBaseline) startRemote(s *testkit.Scenario) {
	s.Precondition("start the remote Waku node and retain a private sentinel", func(t *testing.T) {
		started := time.Now()
		require.NoError(t, b.remote.Start(b.ctx))
		b.remoteStartup = time.Since(started)
		require.LessOrEqual(t, b.remoteStartup, startupLimit)
		var err error
		b.sentinel, err = b.sender.Seal(networkprivacy.MessageClassDiscoveryRecord, 1, []byte("performance-store-sentinel"))
		require.NoError(t, err)
		require.NoError(t, b.remote.PublishPrivateEnvelope(b.ctx, b.sentinel))
		b.events, err = b.remote.SubscribeRelayEnvelopes(b.ctx, transport.DefaultPubsubTopic, b.contentTopic)
		require.NoError(t, err)
	})
}

func (b *performanceBaseline) startLocal(s *testkit.Scenario) {
	s.Step("start and join the local Waku node within the startup bound", func(t *testing.T) {
		started := time.Now()
		b.local.SetBootstrapNodes(b.remote.Endpoints())
		require.NoError(t, b.local.Start(b.ctx))
		testkit.WaitForRelayReadiness(t, b.local)
		b.localReadiness = time.Since(started)
		require.LessOrEqual(t, b.localReadiness, startupLimit)
	})
}

func (b *performanceBaseline) checkConnections(s *testkit.Scenario) {
	s.Assert("peer and relay connection counts stay bounded", func(t *testing.T) {
		b.connections["local_peers"] = b.local.PeerCount()
		b.connections["remote_peers"] = b.remote.PeerCount()
		b.connections["local_relay"] = b.local.RelayPeerCount(transport.DefaultPubsubTopic)
		b.connections["remote_relay"] = b.remote.RelayPeerCount(transport.DefaultPubsubTopic)
		for label, count := range b.connections {
			assertConnectionBound(t, label, count)
		}
	})
}

func (b *performanceBaseline) deliverBatch(s *testkit.Scenario) {
	s.Step("deliver the bounded encrypted relay batch", func(t *testing.T) {
		published := make(map[string]time.Time, performanceBatchSize)
		batchStarted := time.Now()
		for i := 0; i < performanceBatchSize; i++ {
			payload := bytes.Repeat([]byte{byte(i + 1)}, 1024)
			sealed, err := b.sender.Seal(networkprivacy.MessageClassDiscoveryRecord, 1, payload)
			require.NoError(t, err)
			published[string(sealed.Payload)] = time.Now()
			require.NoError(t, b.local.PublishPrivateEnvelope(b.ctx, sealed))
		}
		latencies := collectPerformanceBatch(t, b.ctx, b.events, published)
		b.batchDuration = time.Since(batchStarted)
		b.deliveryP95 = percentile95(latencies)
		b.throughput = float64(performanceBatchSize) / b.batchDuration.Seconds()
	})
}

func (b *performanceBaseline) checkRelayMetrics(s *testkit.Scenario) {
	s.Assert("relay throughput and latency meet release thresholds", func(t *testing.T) {
		require.LessOrEqual(t, b.batchDuration, batchLimit)
		require.LessOrEqual(t, b.deliveryP95, deliveryP95Limit)
		require.GreaterOrEqual(t, b.throughput, 3.0)
	})
}

func (b *performanceBaseline) fetchStore(s *testkit.Scenario) {
	s.Assert("Store returns the retained sentinel within the query bound", func(t *testing.T) {
		started := time.Now()
		items, err := b.local.FetchPrivateEnvelopes(b.ctx, b.remote.Endpoints(), b.contentTopic)
		b.storeDuration = time.Since(started)
		require.NoError(t, err)
		require.LessOrEqual(t, b.storeDuration, storeQueryLimit)
		require.True(t, containsSealedPayload(items, b.sentinel.Payload), "retained sentinel missing from Store result")
	})
}

func (b *performanceBaseline) stopTransports(s *testkit.Scenario) {
	s.Step("stop both Waku nodes within the shutdown bound", func(t *testing.T) {
		b.localShutdown = stopPerformanceTransport(t, b.local)
		b.localStopped = true
		b.remoteShutdown = stopPerformanceTransport(t, b.remote)
		b.remoteStopped = true
	})
}

func (b *performanceBaseline) logMetrics(s *testkit.Scenario) {
	s.Assert("all measured values remain inside PERF-01", func(t *testing.T) {
		t.Logf("PERF-01 remote_start=%s local_ready=%s local_peers=%d remote_peers=%d local_relay=%d remote_relay=%d batch=%s throughput=%.2f_msg_s p95=%s store=%s local_stop=%s remote_stop=%s",
			b.remoteStartup, b.localReadiness, b.connections["local_peers"], b.connections["remote_peers"],
			b.connections["local_relay"], b.connections["remote_relay"], b.batchDuration, b.throughput,
			b.deliveryP95, b.storeDuration, b.localShutdown, b.remoteShutdown)
	})
}

func newPerformanceTransport(t *testing.T, name string) transport.Service {
	t.Helper()
	dir := t.TempDir()
	return transport.New(transport.Config{
		NodeProfile:      transport.NodeProfileServiceNode,
		Profile:          transport.ProfileTCPOnly,
		ReachabilityMode: transport.ReachabilityLocalOnly,
		StorePath:        filepath.Join(dir, name+"-store.db"),
		PrivateKeyPath:   filepath.Join(dir, name+"-key.json"),
	})
}

func assertConnectionBound(t *testing.T, label string, count int) {
	t.Helper()
	require.GreaterOrEqual(t, count, 1, label)
	require.LessOrEqual(t, count, 4, label)
}

func collectPerformanceBatch(t *testing.T, ctx context.Context, events <-chan transport.Envelope, published map[string]time.Time) []time.Duration {
	t.Helper()
	latencies := make([]time.Duration, 0, len(published))
	for len(latencies) < len(published) {
		select {
		case event, ok := <-events:
			require.True(t, ok, "relay subscription closed before batch completion")
			started, wanted := published[string(event.Payload)]
			if !wanted {
				continue
			}
			latencies = append(latencies, time.Since(started))
			delete(published, string(event.Payload))
		case <-ctx.Done():
			require.FailNow(t, "relay batch deadline exceeded", fmt.Sprintf("missing=%d: %v", len(published), ctx.Err()))
		}
	}
	return latencies
}

func percentile95(values []time.Duration) time.Duration {
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	index := int(math.Ceil(float64(len(values))*0.95)) - 1
	return values[index]
}

func containsSealedPayload(items []networkprivacy.SealedEnvelope, expected []byte) bool {
	for _, item := range items {
		if bytes.Equal(item.Payload, expected) {
			return true
		}
	}
	return false
}

func stopPerformanceTransport(t *testing.T, svc transport.Service) time.Duration {
	t.Helper()
	started := time.Now()
	stopCtx, cancel := context.WithTimeout(context.Background(), shutdownLimit)
	defer cancel()
	require.NoError(t, svc.Stop(stopCtx))
	duration := time.Since(started)
	require.LessOrEqual(t, duration, shutdownLimit)
	return duration
}
