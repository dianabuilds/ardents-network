//go:build e2e

package networkprivacye2e_test

import (
	"context"
	"testing"
	"time"

	networkapi "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestExternalWakuCaptureContainsOnlyPrivateCarrierShape(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerE2E, Domain: "network-privacy", ScenarioID: "E2E-NPI-001",
		Suite: "e2e", Tags: []string{"e2e", "network", "privacy", "waku", "capture"},
		Speed: "default", Environment: "local",
	})
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	now := time.Now().UTC().Truncate(time.Second)
	fixture := testkit.NewDiscoveryPrivacyFixture(t, now)
	plaintext := []byte(`{"principal":"p_visible","service":"svc_visible","operation":"publish"}`)

	receiver := testkit.StartTransport(t)
	topic, err := fixture.Receiver.ContentTopic()
	require.NoError(t, err)
	captures, err := receiver.SubscribeRelayEnvelopes(ctx, networkapi.DefaultPubsubTopic, topic)
	require.NoError(t, err)
	sender := testkit.StartBootstrappedTransport(t, receiver)
	testkit.WaitForRelayReadiness(t, sender)

	scenario.Step("publish a product message through the private channel and real Waku carrier", func(t *testing.T) {
		sealed, sealErr := fixture.Sender.Seal(networkprivacy.MessageClassDiscoveryRecord, 1, plaintext)
		require.NoError(t, sealErr)
		require.NoError(t, sender.PublishPrivateEnvelope(ctx, sealed))
	})

	scenario.Assert("an external carrier capture exposes neither routing nor payload meaning", func(t *testing.T) {
		select {
		case captured := <-captures:
			findings := testkit.InspectPrivateCapture(
				captured, plaintext, []byte("p_visible"), []byte("svc_visible"), []byte("publish"),
			)
			require.Empty(t, findings)
		case <-ctx.Done():
			require.FailNow(t, "external Waku capture timed out")
		}
	})
}
