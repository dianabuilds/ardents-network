//go:build integration

package network_test

import (
	"testing"
	"time"

	networkprivacy "ardents/internal/messaging"
	transport "ardents/internal/network"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestTransportRelayPublishSubscribeAndStoreFetch(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "network-foundation",
		ScenarioID:  "NFI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "network"},
		Speed:       "default",
		Environment: "local",
	})

	ctx := t.Context()
	var remote transport.Service
	var local transport.Service
	var events <-chan transport.Envelope
	var now time.Time
	var senderChannel *networkprivacy.Channel
	var contentTopic string

	scenario.Precondition("start remote transport and subscribe to relay topic", func(t *testing.T) {
		remote = testkit.StartTransport(t)
		now = time.Now().UTC().Truncate(time.Second)
		fixture := testkit.NewDiscoveryPrivacyFixture(t, now)
		senderChannel, _ = fixture.Sender, fixture.Receiver
		var err error
		contentTopic, err = senderChannel.ContentTopic()
		require.NoError(t, err)
		events, err = remote.SubscribeRelayEnvelopes(ctx, transport.DefaultPubsubTopic, contentTopic)
		require.NoError(t, err)
	})

	scenario.Step("publish opaque private envelope into remote store", func(t *testing.T) {
		sealed, err := senderChannel.Seal(networkprivacy.MessageClassDiscoveryRecord, 1, []byte("stored-private-record"))
		require.NoError(t, err)
		require.NoError(t, remote.PublishRelayEnvelope(ctx, carrierEnvelope(sealed)))
	})

	scenario.Step("bootstrap local transport and publish relay payload", func(t *testing.T) {
		local = testkit.StartBootstrappedTransport(t, remote)
		testkit.WaitForRelayReadiness(t, local)

		sealed, err := senderChannel.Seal(networkprivacy.MessageClassDiscoveryRecord, 1, []byte("hello-private-relay"))
		require.NoError(t, err)
		require.NoError(t, local.PublishRelayEnvelope(ctx, carrierEnvelope(sealed)))
	})

	scenario.Assert("remote subscriber receives relay envelope", func(t *testing.T) {
		select {
		case item, ok := <-events:
			require.True(t, ok)
			require.Equal(t, contentTopic, item.ContentTopic)
			require.NotContains(t, item.Payload, []byte("hello-private-relay"))
		case <-ctx.Done():
			require.FailNow(t, "timed out waiting for relay envelope")
		}
	})

	scenario.Assert("store fetch returns only opaque private envelopes", func(t *testing.T) {
		testkit.WaitForCondition(t, 5*time.Second, "private store fetch from remote bootstrap peer", func() (bool, string) {
			envelopes, err := local.FetchEnvelopes(ctx, remote.Endpoints(), contentTopic)
			if err != nil {
				return false, err.Error()
			}
			if len(envelopes) == 0 {
				return false, "unexpected record count"
			}
			for _, envelope := range envelopes {
				if string(envelope.Payload) == "stored-private-record" {
					return false, "store exposed plaintext"
				}
			}
			return true, ""
		})
	})
}
