//go:build integration

package networkfoundation_test

import (
	"context"
	"testing"
	"time"

	datatransfer "ardents/internal/data/transfer"
	networkapi "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestPrivateDataRequestHidesRoutingAndContentIdentity(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "network-foundation", ScenarioID: "NPI-004",
		Suite: "integration", Tags: []string{"integration", "network", "privacy", "data"},
		Speed: "default", Environment: "local",
	})
	ctx := t.Context()
	now := time.Now().UTC().Truncate(time.Second)
	privacy := testkit.NewDataPrivacyFixture(t, now)
	receiver := testkit.StartTransport(t)
	topic, err := privacy.Receiver.ContentTopic()
	require.NoError(t, err)
	captured, err := receiver.SubscribeRelayEnvelopes(ctx, networkapi.DefaultPubsubTopic, topic)
	require.NoError(t, err)
	sender := testkit.StartBootstrappedTransport(t, receiver)
	testkit.WaitForRelayReadiness(t, sender)

	payload := []byte(`{"request_id":"request-visible","blob_id":"blob-visible","requester":"principal-visible"}`)
	exchange := datatransfer.NewPrivateExchange(privacy.Sender, sender)
	require.NoError(t, exchange.Publish(ctx, networkprivacy.MessageClassBlobFetchRequest, payload))

	select {
	case envelope := <-captured:
		require.Equal(t, topic, envelope.ContentTopic)
		require.NotContains(t, envelope.ContentTopic, "blob")
		require.NotContains(t, envelope.Payload, []byte("request-visible"))
		require.NotContains(t, envelope.Payload, []byte("blob-visible"))
		require.NotContains(t, envelope.Payload, []byte("principal-visible"))
		require.Empty(t, testkit.InspectPrivateCapture(
			envelope, []byte("blob"), []byte("request-visible"), []byte("blob-visible"), []byte("principal-visible"),
		))
		sealed := networkprivacy.SealedEnvelope{
			PubsubTopic: envelope.PubsubTopic, ContentTopic: envelope.ContentTopic, Payload: envelope.Payload,
		}
		opened, openErr := privacy.Receiver.Open(sealed)
		require.NoError(t, openErr)
		require.Equal(t, payload, opened.Payload)
		_, replayErr := privacy.Receiver.Open(sealed)
		require.Equal(t, networkprivacy.CodeEnvelopeReplayed, networkprivacy.CodeOf(replayErr))
	case <-ctx.Done():
		require.FailNow(t, "private data request was not captured")
	}
}

func TestPrivateDataExchangeRejectsRevokedRequesterCapability(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "network-foundation", ScenarioID: "NPI-004",
		Suite: "integration", Tags: []string{"integration", "network", "privacy", "data", "revocation"},
		Speed: "default", Environment: "local",
	})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	now := time.Now().UTC().Truncate(time.Second)
	privacy := testkit.NewDataPrivacyFixture(t, now)
	receiver := testkit.StartTransport(t)
	receiverExchange := datatransfer.NewPrivateExchange(privacy.Receiver, receiver)
	require.NoError(t, receiverExchange.Start(ctx))
	sender := testkit.StartBootstrappedTransport(t, receiver)
	testkit.WaitForRelayReadiness(t, sender)
	privacy.RevokeSender(t, now)

	senderExchange := datatransfer.NewPrivateExchange(privacy.Sender, sender)
	require.NoError(t, senderExchange.Publish(ctx, networkprivacy.MessageClassBlobFetchRequest, []byte(`{"request_id":"revoked"}`)))

	select {
	case err := <-receiverExchange.Failures():
		require.Equal(t, networkprivacy.CodeEnvelopeSenderUnauthorized, networkprivacy.CodeOf(err))
	case <-ctx.Done():
		require.FailNow(t, "revoked private data request was not rejected")
	}
}
