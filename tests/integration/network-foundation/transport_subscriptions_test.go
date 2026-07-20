//go:build integration

package networkfoundation_test

import (
	"testing"

	transport "ardents/internal/network/api"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestTransportSubscriptionDeliversMultipleContentTopics(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "network-foundation",
		ScenarioID:  "NFI-003",
		Suite:       "integration",
		Tags:        []string{"integration", "network", "relay"},
		Speed:       "default",
		Environment: "local",
	})

	ctx := t.Context()
	var remote transport.Service
	var local transport.Service
	var events <-chan transport.Envelope

	scenario.Precondition("start remote subscriber for two content topics", func(t *testing.T) {
		remote = testkit.StartTransport(t)
		var err error
		events, err = remote.SubscribeRelayEnvelopes(ctx, transport.DefaultPubsubTopic, "ardents/1/test/a", "ardents/1/test/b")
		require.NoError(t, err)
	})

	scenario.Step("start bootstrapped local transport and wait for relay readiness", func(t *testing.T) {
		local = testkit.StartBootstrappedTransport(t, remote)
		testkit.WaitForRelayReadiness(t, local)
	})

	scenario.Step("publish both content topics", func(t *testing.T) {
		require.NoError(t, local.PublishRelayEnvelope(ctx, transport.Envelope{
			PubsubTopic:  transport.DefaultPubsubTopic,
			ContentTopic: "ardents/1/test/a",
			Payload:      []byte("first"),
		}))
		require.NoError(t, local.PublishRelayEnvelope(ctx, transport.Envelope{
			PubsubTopic:  transport.DefaultPubsubTopic,
			ContentTopic: "ardents/1/test/b",
			Payload:      []byte("second"),
		}))
	})

	scenario.Assert("subscription receives both content topics", func(t *testing.T) {
		got := map[string]string{}
		for len(got) < 2 {
			select {
			case env, ok := <-events:
				require.True(t, ok)
				got[env.ContentTopic] = string(env.Payload)
			case <-ctx.Done():
				require.FailNowf(t, "timed out waiting for topics", "received=%v", got)
			}
		}
		require.Equal(t, "first", got["ardents/1/test/a"])
		require.Equal(t, "second", got["ardents/1/test/b"])
	})
}
